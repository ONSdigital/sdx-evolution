package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/ONSdigital/sdx-onyx-gazelle/lib/api"
	"github.com/gorilla/mux"
)

func main() {
	var port string
	if port = os.Getenv("PORT"); len(port) == 0 {
		log.Fatal(`event="Failed to start - Must supply PORT environment variable"`)
	}

	r := mux.NewRouter()
	r.HandleFunc("/healthcheck", HealthcheckHandler).Methods("GET")
	r.HandleFunc("/survey", StorePostedSurvey).Methods("POST")
	http.Handle("/", r)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

// StorePostedSurvey attempts to place the given survey data into the data store
func StorePostedSurvey(rw http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf(`event="Failed to read posted data" error="%v"`, err)
		api.WriteProblemResponse(api.Problem{
			Title:  "Failed to read posted data",
			Status: http.StatusBadRequest,
		}, rw)
		return
	}

	var survey Survey
	if err = json.Unmarshal(body, &survey); err != nil {
		log.Printf(`event="Failed to parse posted data" error="%v"`, err)
		api.WriteProblemResponse(api.Problem{
			Title:  "Failed to parse posted data",
			Status: http.StatusBadRequest,
		}, rw)
		return
	}

	log.Printf(`event="Would be attempting to store survey" tx_id="%s"`, survey.TxID)

	rw.WriteHeader(http.StatusOK)
}
