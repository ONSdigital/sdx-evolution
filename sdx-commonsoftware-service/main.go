package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ONSdigital/sdx-onyx-gazelle/lib/rabbit"
	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

var (
	downstreamExchange string
	rabbitConn         *amqp.Connection
)

const (
	topic = "survey.downstream.commonsoftware.#"
)

func main() {
	var port string
	if port = os.Getenv("PORT"); len(port) == 0 {
		log.Fatal(`event="Failed to start - Must supply PORT environment variable"`)
	}

	var rabbitURI string
	if rabbitURI = os.Getenv("RABBIT_URL"); len(rabbitURI) == 0 {
		log.Fatal(`event="Failed to start - Must supply RABBIT_URL environment variable"`)
	}
	if !strings.HasPrefix(rabbitURI, "amqp://") {
		log.Fatal(`event="Failed to start - RABBIT_URL must contain amqp:// prefix`)
	}

	if downstreamExchange = os.Getenv("DOWNSTREAM_EXCHANGE"); len(downstreamExchange) == 0 {
		log.Fatal(`event="Failed to start - DOWNSTREAM_EXCHANGE must be specified"`)
	}

	rabbitConn = rabbit.ConnectWithRetry(rabbitURI, time.Second*2)
	defer rabbitConn.Close()

	cancel, err := rabbit.StartSimpleTopicConsumer(
		downstreamExchange,
		topic,
		"sdx.survey.downstream.cs.work",
		rabbitConn,
		worker,
	)
	if err != nil {
		log.Fatalf(`event="Failed to start incoming queue" error="%v"`, err)
	}
	defer cancel()

	// WEBSERVER

	r := mux.NewRouter()
	r.HandleFunc("/healthcheck", HealthcheckHandler).Methods("GET")
	http.Handle("/", r)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func worker(b []byte) {
	log.Printf(`event="Would be downstreaming (CS)" tx_id="%s"`, b)
}
