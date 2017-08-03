package main

import (
	"context"
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
	rabbitConn     *amqp.Connection
	legacyExchange string
	notifyExchange string
)

const (
	// Default queue topic
	Topic = "survey.#"
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

	if notifyExchange = os.Getenv("NOTIFICATION_EXCHANGE"); len(notifyExchange) == 0 {
		log.Fatal(`event="Failed to start - NOTIFICATION_EXCHANGE must be specified"`)
	}
	if legacyExchange = os.Getenv("LEGACY_EXCHANGE"); len(legacyExchange) == 0 {
		log.Fatal(`event="Failed to start - LEGACY_EXCHANGE must be specified"`)
	}

	rabbitConn = rabbit.ConnectWithRetry(rabbitURI, time.Second*2)
	defer rabbitConn.Close()

	cancel, err := startQueues(rabbitConn)
	if err != nil {
		log.Fatalf(`event="Failed to start incomming queue" error="%v"`, err)
	}
	defer cancel()

	r := mux.NewRouter()
	r.HandleFunc("/healthcheck", HealthcheckHandler).Methods("GET")
	http.Handle("/", r)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func startQueues(conn *amqp.Connection) (func(), error) {
	var err error

	// Create incoming and outgoing channels
	chIn, err := conn.Channel()
	if err != nil {
		log.Printf(`event="Failed to create incoming channel" error="%s"`, err)
	}

	chOut, err := conn.Channel()
	if err != nil {
		log.Printf(`event="Failed to create outgoing channel" error="%s"`, err)
	}

	// Declare the dead letter exchange for the work exchange
	var deadLetterExchange string
	if deadLetterExchange, err = rabbit.DeclareDeadLetterExchangeWithDefaults(legacyExchange, chOut); err != nil {
		return nil, err
	}

	// Declare the work exchange + incoming queue
	// This binds back to the notification exchange
	if err = rabbit.DeclareExchangeWithDefaults(legacyExchange, chIn); err != nil {
		return nil, err
	}

	if err = rabbit.DeclareExchangeWithDefaults(notifyExchange, chIn); err != nil {
		return nil, err
	}

	if err := chIn.ExchangeBind(legacyExchange, Topic, notifyExchange, false, nil); err != nil {
		log.Printf(`event="Failed to bind exchanges" error="%v"`, err)
		return nil, err
	}

	qWork, err := chIn.QueueDeclare("legacy.work", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	if err = chIn.QueueBind(qWork.Name, Topic, legacyExchange, false, nil); err != nil {
		return nil, err
	}

	msgs, err := chIn.Consume(qWork.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf(`event="Failed to start consuming queue" error="%v"`, err)
		return nil, err
	}

	// Start up the delay queue
	qDelay, err := chOut.QueueDeclare(
		"legacy.delay",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange": legacyExchange,
			"x-message-ttl":          int16(5000),
			// "x-dead-letter-routing-key": Topic, // Let the individual publish set this
		},
	)
	if err != nil {
		log.Printf(`event="Failed to declare delay queue" error="%v"`, err)
		return nil, err
	}

	if err = chOut.QueueBind(qDelay.Name, "survey.#", deadLetterExchange, false, nil); err != nil {
		log.Printf(`event="Failed to bind queue" error="%v"`, err)
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	i := 0

	go func(ctx context.Context) {
		for d := range msgs {
			select {
			case <-ctx.Done():
				log.Print(`event="Canceling consumer"`)
				chOut.Close()
			default:
				log.Printf(`event="Legacy router received message" data="%s"`, d.Body)
				log.Print(`event="RE-PUBLISHING MESSAGE!"`)

				// if err = d.Nack(false, false); err != nil {
				// 	// TODO How to handle better
				// 	log.Fatalf(`event="Failed to nack"`)
				// }

				if err = d.Reject(false); err != nil {
					log.Fatalf("FAILED TO REJECT MESSAGE %v", err) // TODO
				}

				i++

				log.Printf("Routing key is %s", d.RoutingKey)

				if err = chOut.Publish(
					deadLetterExchange,
					d.RoutingKey,
					false,
					false,
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        d.Body,
						Headers: amqp.Table{
							// 	"x-retry-count":             int16(i),
							// 	"x-dead-letter-exchange":    legacyExchange,
							"x-dead-letter-routing-key": d.RoutingKey,
						},
					}); err != nil {

					// TODO How to properly handle error here
					log.Fatalf(`event="Failed to publish to delay queue" error="%v"`, err)
				}
			}
		}
	}(ctx)
	log.Print(`event="Started consumer"`)

	return cancel, nil
}
