package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ONSdigital/sdx-evolution/cmd/sdx-legacy-router-service/config"
	"github.com/ONSdigital/sdx-evolution/internal/rabbit"
	redis "github.com/ONSdigital/sdx-evolution/internal/redis"
	"github.com/ONSdigital/sdx-evolution/internal/signals"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

var (
	rabbitConn *amqp.Connection
	redisConn  redis.Conn
)

// Various constants
const (
	// Delay is the amount of time (milliseconds) to delay a message
	// when we want to defer processing.
	// The int16 type is required as rabbitmq headers need to be explicitly sized
	// as int16, int32 or int64 for ints
	delay = int16(5000)

	RedisURL = "redis://redis:6379"
)

// Queues and topics
const (
	workQueue  = "sdx.survey.legacy.work"
	delayQueue = "sdx.survey.legacy.delay"

	workQueueTopic  = "survey.#"
	delayQueueTopic = "survey.#"
)

func main() {

	config.Load()

	cancelSigWatch := signals.HandleFunc(
		func(sig os.Signal) {
			log.Printf(`event="Shutting down" signal="%s"`, sig.String())
			if rabbitConn != nil {
				log.Printf(`event="Closing rabbit connection"`)
				rabbitConn.Close()
			}
			if redisConn != nil {
				log.Printf(`event="Closing redis connection"`)
				redisConn.Close()
			}
			log.Print(`event="Exiting"`)
			os.Exit(0)
		},
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer cancelSigWatch()

	// Cache (Redis)
	redisConn = redis.ConnectWithRetry(RedisURL, time.Second*2)
	defer redisConn.Close()

	if err := populateInitialSurveyConfig(); err != nil {
		log.Printf(`event="Redis error" error="%s"`, err)
	}

	// RabbitMQ
	rabbitConn = rabbit.ConnectWithRetry(config.C["RABBIT_URL"], time.Second*2)
	defer rabbitConn.Close()

	cancel, err := startQueues(rabbitConn)
	if err != nil {
		log.Fatalf(`event="Failed to start incomming queue" error="%v"`, err)
	}
	defer cancel()

	// Webserver
	healthCheckCancel, err := StartHealthChecking()
	if err != nil {
		log.Fatalf(`event="Failed to start - can't set health" error="%s"`, err)
	}
	defer healthCheckCancel()

	r := mux.NewRouter()
	r.HandleFunc("/healthcheck", HealthcheckHandler).Methods("GET")
	http.Handle("/", r)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%s", config.C["PORT"]), nil))
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
	var legacyDelayExchange string
	if legacyDelayExchange, err = rabbit.DeclareDeadLetterExchangeWithDefaults(config.C["LEGACY_EXCHANGE"], chOut); err != nil {
		return nil, err
	}

	// Declare the work exchange + incoming queue
	// This binds back to the notification exchange
	for _, e := range []string{"LEGACY_EXCHANGE", "NOTIFICATION_EXCHANGE"} {
		if err = rabbit.DeclareExchangeWithDefaults(e, chIn); err != nil {
			log.Printf(`event="Failed to declare exchange" exchange="%s" error="%v"`, e, err)
			return nil, err
		}
	}
	// if err = rabbit.DeclareExchangeWithDefaults(config.C["LEGACY_EXCHANGE"], chIn); err != nil {
	// 	return nil, err
	// }

	// if err = rabbit.DeclareExchangeWithDefaults(config.C["NOTIFICATION_EXCHANGE"], chIn); err != nil {
	// 	return nil, err
	// }

	if err := chIn.ExchangeBind(config.C["LEGACY_EXCHANGE"], workQueueTopic, config.C["NOTIFICATION_EXCHANGE"], false, nil); err != nil {
		log.Printf(`event="Failed to bind exchanges" error="%v"`, err)
		return nil, err
	}

	// Declare the downstream exchange
	if err = rabbit.DeclareExchangeWithDefaults(config.C["DOWNSTREAM_EXCHANGE"], chOut); err != nil {
		return nil, err
	}

	// The work queue is bound to the legacy exchange and is used to receive
	// messages for processing.
	qWork, err := chIn.QueueDeclare(workQueue, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	if err = chIn.QueueBind(qWork.Name, workQueueTopic, config.C["LEGACY_EXCHANGE"], false, nil); err != nil {
		return nil, err
	}

	msgs, err := chIn.Consume(qWork.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf(`event="Failed to start consuming queue" error="%v"`, err)
		return nil, err
	}

	// Start up the delay queue
	qDelay, err := chOut.QueueDeclare(
		delayQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange": config.C["LEGACY_EXCHANGE"],
			"x-message-ttl":          delay,
			// The x-dead-letter-routing-key is set on a per-message basis
			// as and when it is published to the delay exchange.
		},
	)
	if err != nil {
		log.Printf(`event="Failed to declare delay queue" error="%v"`, err)
		return nil, err
	}

	if err = chOut.QueueBind(qDelay.Name, delayQueueTopic, legacyDelayExchange, false, nil); err != nil {
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
				surveyConfig, err := getSurveyConfig()
				if err != nil {
					log.Fatalf(`event="Failed to get survey config" error="%s"`, err) // TODO
				}
				log.Println(surveyConfig.Surveys)

				// Assuming routing key is survey.notify.<source>.<type>
				surveyID := strings.Split(d.RoutingKey, ".")[3]
				surveyIsActive := false

				// TODO This is assuming the survey config for this survey
				// 		exists - really it should check and if not found
				//		should assume it's not active
				var thisSurvey Survey
				if val, ok := surveyConfig.Surveys[surveyID]; ok {
					thisSurvey = val
					surveyIsActive = thisSurvey.Active
				}

				if surveyIsActive {

					// TOPIC: survey.downstream.<downstream>.<survey_id>
					downstreamRoutingKey := fmt.Sprintf("survey.downstream.%s.%s", thisSurvey.Downstream, surveyID)

					log.Print(`event="Survey is active - processing"`)

					if err = chOut.Publish(
						config.C["DOWNSTREAM_EXCHANGE"],
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
