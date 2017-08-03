package rabbit

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
)

// ConnectWithRetry will repeatedly try to connect to a rabbit instance at the
// specified intervals
func ConnectWithRetry(uri string, retryInterval time.Duration) (conn *amqp.Connection) {
	var err error
	for {
		conn, err = amqp.Dial(uri)
		if err == nil {
			log.Println(`event="Established connection to rabbit"`)
			return conn
		}
		log.Printf(`event="Failed to connect to rabbit - retrying" err="%v"`, err)
		time.Sleep(retryInterval)
	}
}

// DeclareExchangeWithDefaults attempts to declare a given named exchange with
// a set of default values.
func DeclareExchangeWithDefaults(exchange string, ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange [%s]: %v", exchange, err)
	}
	return nil
}

// DeclareDeadLetterExchangeWithDefaults creates a dead-letter exchange based on
// the exchange given (e.g. if exchange="hello" will create "hello.dead.letter")
func DeclareDeadLetterExchangeWithDefaults(exchange string, ch *amqp.Channel) (string, error) {
	name := fmt.Sprintf("%s.dead.letter", exchange)

	if err := ch.ExchangeDeclare(
		name,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange": exchange,
			"x-message-ttl":          int16(5000),
		},
	); err != nil {
		return "", fmt.Errorf("failed to declare dead letter exchange [%s]: %v", name, err)
	}
	return name, nil
}

// StartSimpleTopicConsumer attempts to start consuming from the given topic
// and exchange with a supplied processing function. If successful it returns
// the context cancel function for the go routine it spawns.
func StartSimpleTopicConsumer(exchange, topic, queueName string, conn *amqp.Connection, work func([]byte)) (func(), error) {
	if conn == nil {
		return nil, errors.New("No rabbit connection supplied")
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	if err = ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return nil, err
	}

	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	if err = ch.QueueBind(q.Name, topic, exchange, false, nil); err != nil {
		return nil, err
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	// Create the cancel context
	ctx, cancel := context.WithCancel(context.Background())

	go func(ctx context.Context) {
		for d := range msgs {
			select {
			case <-ctx.Done():
				log.Print(`event="Canceling consumer"`)
				ch.Close()
			default:
				work(d.Body)
			}
		}
	}(ctx)
	log.Print(`event="Started consumer"`)

	return cancel, nil
}
