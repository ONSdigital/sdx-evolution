package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ONSdigital/sdx-evolution/internal/rabbit"
	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

var (
	rabbitConn     *amqp.Connection
	notifyExchange string
)

func main() {
	// var err error

	var port string
	if port = os.Getenv("PORT"); len(port) == 0 {
		log.Fatal(`event="Failed to start - Must supply PORT environment variable"`)
	}

	var rabbitURI string
	if rabbitURI = os.Getenv("RABBIT_URL"); len(rabbitURI) == 0 {
		log.Fatal(`event="Failed to start - Must supply RABBIT_URL environment variable"`)
	}
	if !strings.HasPrefix(rabbitURI, "amqp://") {
		log.Fatal(`event="Failed to start - RABBITURL must contain amqp:// prefix`)
	}

	if notifyExchange = os.Getenv("NOTIFICATION_EXCHANGE"); len(notifyExchange) == 0 {
		log.Fatal(`event="Failed to start - NOTIFICATION_EXCHANGE must be specified"`)
	}

	// Set up the connection to the RabbitMQ server
	rabbitConn = rabbit.ConnectWithRetry(rabbitURI, time.Second*2)
	defer rabbitConn.Close()

	// Define the worker function that will process messages received from
	// the queue
	worker := func(b []byte) {
		log.Printf(`event="Would be receipting" tx_id="%s"`, b)
	}

	// Start up the worker to consume from the queue
	cancel, err := rabbit.StartSimpleTopicConsumer(notifyExchange, "survey.#", "test_survey_receipt", rabbitConn, worker)
	if err != nil {
		log.Fatalf(`event="Failed to start consumer" error="%v"`, err)
	}
	defer cancel()

	r := mux.NewRouter()

	r.HandleFunc("/healthcheck", HealthcheckHandler).Methods("GET")

	http.Handle("/", r)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
