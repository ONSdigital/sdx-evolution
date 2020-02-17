package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/ONSdigital/sdx-evolution/cmd/sdx-survey-gateway-service/config"
	"github.com/ONSdigital/sdx-evolution/internal/api"
	"github.com/ONSdigital/sdx-evolution/internal/rabbit"
	"github.com/ONSdigital/sdx-evolution/internal/signals"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

var (
	rabbitConn *amqp.Connection
)

func main() {

	config.Load()

	// Set up the signal handler to watch for SIGTERM and SIGINT signals so we
	// can at least attempt to gracefully shut down before the PaaS/docker etc
	// running us unceremoneously kills us with a SIGKILL.
	cancelSigWatch := signals.HandleFunc(
		func(sig os.Signal) {
			log.Printf(`event="Shutting down" signal="%s"`, sig.String())
			if rabbitConn != nil {
				log.Printf(`event="Closing rabbit connection"`)
				rabbitConn.Close()
			}
			log.Print(`event="Exiting"`)
			os.Exit(0)
		},
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer cancelSigWatch()

	// RabbitMQ
	rabbitConn = rabbit.ConnectWithRetry(config.C["RABBIT_URL"], time.Second*2)
	defer rabbitConn.Close()

	// Webserver
	r := mux.NewRouter()
	r.HandleFunc("/healthcheck", HealthcheckHandler).Methods("GET")
	r.HandleFunc("/survey", PostedSurveyHandler).Methods("POST")
	http.Handle("/", r)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%s", config.C["PORT"]), nil))
}

// PostedSurveyHandler takes posted survey data (encrypted) and processes it
func PostedSurveyHandler(rw http.ResponseWriter, r *http.Request) {

	// Grab the posted data from the client. This WILL be an encrypted blob
	// of JWS lovelyness, but for now we're skipping that step and assuming
	// it's already decrypted by using plain json.
	//
	// nb. Don't need to close the request body - the server does that for
	//     us - https://golang.org/pkg/net/http/#Request
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf(`event="Failed to read posted data" error="%v"`, err)
		api.WriteProblemResponse(api.Problem{
			Title:  "Request body unreadable",
			Status: http.StatusInternalServerError,
		}, rw)
		return
	}
	if len(body) == 0 {
		log.Print(`event="Failed to read posted data" error="Body is empty"`)
		api.WriteProblemResponse(api.Problem{
			Title:  "Request body empty",
			Status: http.StatusBadRequest,
		}, rw)
		return
	}

	// Assuming everything is ok with what we receieved (and decrypted)
	// we then go on to:
	//	- store the survey into the datastore (via service)
	// 	- place a notification of the event onto the notify exchange
	var survey Survey
	if err := json.Unmarshal(body, &survey); err != nil {
		log.Printf(`event="Failed to parse survey JSON" error="%v"`, err)
		api.WriteProblemResponse(api.Problem{
			Title:  "Failed to parse survey JSON",
			Status: http.StatusBadRequest,
		}, rw)
		return
	}
	log.Printf(
		`event="Received survey data" survey_id="%s" instrument_id="%s" tx_id="%v"`,
		survey.SurveyID,
		survey.Collection.InstrumentID,
		survey.TxID,
	)

	// Fire to data store
	log.Printf(`event="Attempting to store survey data" tx_id="%s"`, survey.TxID)
	if err = storeSurvey(body); err != nil {
		log.Printf(`event="Failed to store survey JSON" error="%v"`, err)
		api.WriteProblemResponse(api.Problem{
			Title:  "Failed to store survey JSON",
			Status: http.StatusBadRequest,
		}, rw)
		return
	}

	// Notify
	log.Printf(`event="Attempting to publish notification" tx_id="%s"`, survey.TxID)
	if err := publishNotification(survey.TxID, "eq", survey.SurveyID, survey.Collection.InstrumentID); err != nil {
		log.Printf(`event="Failed to publish survey notification event" error="%v"`, err)
		// TODO What happens if we fail to publish?
		//		- Could attempt a few reties?
		//		- Responsibility should be on calling service to handle and retry
		api.WriteProblemResponse(api.Problem{
			Title:  "Failed to notify request",
			Status: http.StatusInternalServerError,
			Detail: "Unable to route survey receipt notification at this time",
		}, rw)
		return
	}

	rw.WriteHeader(http.StatusOK)
}

func storeSurvey(data []byte) error {
	log.Printf(`event="Attempting to store" store="%s"`, config.C["STORE_URL"])
	resp, err := http.Post(config.C["STORE_URL"]+"/survey", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bad response from STORE: %s", resp.Status)
	}
	log.Print(`event="Stored survey"`)
	return nil
}

func publishNotification(id, source, surveyID, instrumentID string) error {

	if rabbitConn == nil {
		return errors.New("No connection to rabbit")
	}

	topic := fmt.Sprintf("survey.notify.%s.%s.%s", source, surveyID, instrumentID)

	// Get a fresh channel for each publish
	// TODO do we need to do this? May be a way of reducing the number of
	//		open/closes (though publishers can't be shared during operation)
	ch, err := rabbitConn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err = rabbit.DeclareExchangeWithDefaults(config.C["NOTIFICATION_EXCHANGE"], ch); err != nil {
		log.Fatalf(`event="Failed to declare exchange" exchange="%s" error="%v"`, config.C["NOTIFICATION_EXCHANGE"], err)
	}

	if err = ch.Publish(
		config.C["NOTIFICATION_EXCHANGE"],
		topic,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(id),
		}); err != nil {
		return err
	}

	log.Printf(`event="Published notification to '%s'"`, topic)
	return nil
}
