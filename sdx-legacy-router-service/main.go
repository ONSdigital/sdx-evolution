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
	"github.com/ONSdigital/sdx-onyx-gazelle/lib/redis"
	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

var (
	rabbitConn          *amqp.Connection
	legacyExchange      string
	legacyDelayExchange string
	downstreamExchange  string
	notifyExchange      string
)

var (
	redisConn redis.Conn
)

// Various constants
const (
	// Topic is the default queue topic that we care about
	Topic = "survey.#"

	// Delay is the amount of time (milliseconds) to delay a message
	// when we want to defer processing.
	// The int16 type is required as rabbitmq headers need to be explicitly sized
	// as int16, int32 or int64 for ints
	Delay = int16(5000)

	RedisURL = "redis://redis:6379"
)

// Queues for consumption and publication
const (
	WorkQueue  = "sdx.survey.legacy.work"
	DelayQueue = "sdx.survey.legacy.delay"
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
	if downstreamExchange = os.Getenv("DOWNSTREAM_EXCHANGE"); len(downstreamExchange) == 0 {
		log.Fatal(`event="Failed to start - DOWNSTREAM_EXCHANGE must be specified"`)
	}

	// REDIS CACHE

	redisConn = redis.ConnectWithRetry(RedisURL, time.Second*2)
	defer redisConn.Close()

	if err := populateInitialSurveyConfig(); err != nil {
		log.Printf(`event="Redis error" error="%s"`, err)
	}

	// QUEUES

	rabbitConn = rabbit.ConnectWithRetry(rabbitURI, time.Second*2)
	defer rabbitConn.Close()

	cancel, err := startQueues(rabbitConn)
	if err != nil {
		log.Fatalf(`event="Failed to start incomming queue" error="%v"`, err)
	}
	defer cancel()

	// WEBSERVER

	r := mux.NewRouter()
	r.HandleFunc("/healthcheck", HealthcheckHandler).Methods("GET")
	http.Handle("/", r)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func startQueues(conn *amqp.Connection) (func(), error) {
	var err error

	// Channel for consuming
	chIn, err := conn.Channel()
	if err != nil {
		log.Printf(`event="Failed to create incoming channel" error="%s"`, err)
	}

	// Channel for publishing
	chOut, err := conn.Channel()
	if err != nil {
		log.Printf(`event="Failed to create outgoing channel" error="%s"`, err)
	}

	// Declare the dead letter exchange for the work exchange
	if legacyDelayExchange, err = rabbit.DeclareDeadLetterExchangeWithDefaults(legacyExchange, chOut); err != nil {
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

	// Declare the downstream exchange
	if err = rabbit.DeclareExchangeWithDefaults(downstreamExchange, chOut); err != nil {
		return nil, err
	}

	// The work queue is bound to the legacy exchange and is used to receive
	// messages for processing.
	qWork, err := chIn.QueueDeclare(WorkQueue, true, false, false, false, nil)
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
		DelayQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange": legacyExchange,
			"x-message-ttl":          Delay,
			// The x-dead-letter-routing-key is set on a per-message basis
			// as and when it is published to the delay exchange.
		},
	)
	if err != nil {
		log.Printf(`event="Failed to declare delay queue" error="%v"`, err)
		return nil, err
	}

	if err = chOut.QueueBind(qDelay.Name, "survey.#", legacyDelayExchange, false, nil); err != nil {
		log.Printf(`event="Failed to bind queue" error="%v"`, err)
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func(ctx context.Context) {
		for d := range msgs {
			select {
			case <-ctx.Done():
				log.Print(`event="Canceling consumer"`)
				chOut.Close()
			default:
				log.Printf(`event="Legacy router received message" data="%s"`, d.Body)

				// Determine whether we want to process this message
				// (for now, ack and discard)
				// or if we want to say "not now" (simulate queue pause)
				config, err := getSurveyConfig()
				if err != nil {
					log.Fatalf(`event="Failed to get survey config" error="%s"`, err) // TODO
				}
				log.Println(config.Surveys)

				// Assuming routing key is survey.notify.<source>.<type>
				keyParts := strings.Split(d.RoutingKey, ".")
				// source := keyParts[2]
				surveyID := keyParts[3]
				// instrumentID := keyParts[4]
				log.Printf("This survey is a : %s", surveyID)

				surveyIsActive := false

				// TODO This is assuming the survey config for this survey
				// 		exists - really it should check and if not found
				//		should assume it's not active
				var thisSurvey Survey
				if val, ok := config.Surveys[surveyID]; ok {
					thisSurvey = val
					surveyIsActive = thisSurvey.Active
				}

				if surveyIsActive {

					// TOPIC: survey.downstream.<downstream>.<survey_id>
					downstreamRoutingKey := fmt.Sprintf("survey.downstream.%s.%s", thisSurvey.Downstream, surveyID)

					log.Print(`event="Survey is active - processing"`)

					if err = chOut.Publish(
						downstreamExchange,
						downstreamRoutingKey,
						false,
						false,
						amqp.Publishing{
							ContentType: "text/plain",
							Body:        d.Body,
						}); err != nil {

						// TODO How to properly handle error here
						log.Fatalf(`event="Failed to publish to downstream queue" error="%v"`, err)
					}

					_ = d.Ack(false)
					continue
				}

				log.Print(`event="Survey is INACTIVE - re-queuing"`)

				// Remove the message from the original queue - amqp prefers
				// a nack() with no requeue over a reject()
				if err = d.Nack(false, false); err != nil {
					// TODO How to handle better
					log.Fatalf(`event="Failed to nack"`)
				}

				if err = chOut.Publish(
					legacyDelayExchange,
					d.RoutingKey,
					false,
					false,
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        d.Body,
						Headers: amqp.Table{
							// Re-publish with the original routing key so that
							// it'll correctly re-route when TTL'd back to the
							// exchange
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
