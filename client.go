package gorabbit

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
	"gitlab.kardinal.ai/coretech/gorabbit/logging"
	"time"
)

var (
	// Connection manages the serialization and deserialization of incoming
	// and outgoing frames and then dispatches them to the correct Channel
	connection *amqp.Connection

	// Channel represents the AMQP channel
	channel *amqp.Channel
)

type MQTTClient interface {
	Disconnect() error
	SendMessage(exchange string, routingKey string, priority uint8, payload []byte) error
	RetryMessage(event *AMQPMessage, maxRetry int) error
	SubscribeToMessages(queue string, consumer string, autoAck bool) (<-chan AMQPMessage, error)
}

type mqttClient struct {
	// Host is the domain name of the RabbitMQ server
	Host string

	// Port is the configured RabbitMQ port, default usually is 5672
	Port uint

	// Username is the the username used when setting up RabbitMQ
	Username string

	// Password is the the password used when setting up RabbitMQ
	Password string

	// Debug is a flag that activates logs for debugging
	debug bool
}

// SendMessage will send the desired payload through the selected channel
// exchange is the name of the exchange targeted for event publishing
// routingKey is the route that the exchange will use to forward the message
// priority is the priority level of the message (1 to 7)
// payload is the object you want to send as a byte array
func (client *mqttClient) SendMessage(exchange string, routingKey string, priority uint8, payload []byte) error {
	// Before sending a message, we need to make sure that Connection and Channel are valid
	if connection == nil || channel == nil {
		// In debug mode, log the warning
		if client.debug {
			logging.Logger.Warn("connection or channel is nil, attempting to reconnect")
		}

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			// In debug mode, log the error
			if client.debug {
				logging.Logger.WithError(err).Error("could not reconnect to rabbitMQ")
			}

			return err
		}
	}

	// Publish the message via the official amqp package
	// with our given configuration
	err := channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        payload,
			Type:        routingKey,
			Priority:    priority,
			MessageId:   uuid.NewString(),
			Headers: map[string]interface{}{
				RedeliveryHeader: 0,
			},
		},
	)

	// In debug mode, log the error
	if err != nil && client.debug {
		logging.Logger.WithError(err).Error("could not redeliver message")
	}

	return err
}

// SubscribeToMessages will connect to a queue and consume all incoming events from it
// Before an event is consumed, it will be parsed via ParseMessage method and then sent back for consumption
// queue is the name of the queue to connect to
// consumer[optional] is the unique identifier of the consumer. Leaving it empty will generate a unique identifier
// if autoAck is set to true, received events will be auto acknowledged as soon as they are consumed (received)
// returns an incoming channel of AMQPMessage (messages)
func (client *mqttClient) SubscribeToMessages(queue string, consumer string, autoAck bool) (<-chan AMQPMessage, error) {
	// Before sending a message, we need to make sure that Connection and Channel are valid
	if connection == nil || channel == nil {
		// In debug mode, log the warning
		if client.debug {
			logging.Logger.Warn("connection or channel is nil, attempting to reconnect")
		}

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			// In debug mode, log the error
			if client.debug {
				logging.Logger.WithError(err).Error("could not reconnect to rabbitMQ")
			}

			return nil, err
		}
	}

	// Consume events via the official amqp package
	// with our given configuration
	messages, err := channel.Consume(
		queue,    // queue
		consumer, // consumer
		autoAck,  // auto ack
		false,    // exclusive
		false,    // no local
		false,    // no wait
		nil,      // args
	)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			logging.Logger.WithFields(logging.LogFields{
				"queue":      queue,
				"consumer":   consumer,
				"autoAck":    autoAck,
				"stacktrace": err,
			}).Error("could not consume rabbitMQ messages")
		}

		return nil, err
	}

	parsedDeliveries := make(chan AMQPMessage)

	go func() {
		for message := range messages {
			// In debug mode, log the event
			if client.debug {
				logging.Logger.WithFields(logging.LogFields{
					"messageId":   message.MessageId,
					"deliverTag":  message.DeliveryTag,
					"redelivered": message.Redelivered,
				}).Info("received amqp delivery")
			}

			parsed, parseErr := ParseMessage(message)

			if parseErr == nil {
				// In debug mode, log the event
				if client.debug {
					logging.Logger.WithFields(logging.LogFields{
						"type":         parsed.Type,
						"microservice": parsed.Microservice,
						"entity":       parsed.Entity,
						"action":       parsed.Action,
					}).Info("AMQP message successfully parsed and sent")
				}

				parsedDeliveries <- *parsed
			} else {
				// In debug mode, log the error
				if client.debug {
					logging.Logger.WithError(parseErr).Error("could not parse AMQP message, sending delivery with empty properties")
				}

				parsedDeliveries <- AMQPMessage{
					Delivery: message,
				}
			}
		}
	}()

	return parsedDeliveries, nil
}

// connect will initialize the AMQP connection, Connection and Channel
// given a configuration
func (client *mqttClient) connect() error {
	dialUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	// In debug mode, log the infos
	if client.debug {
		logging.Logger.WithField("uri", dialUrl).Info("connecting to MQTT server")
	}

	conn, err := amqp.Dial(dialUrl)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			logging.Logger.WithError(err).Info("could not connect to MQTT server")
		}

		return err
	}

	connection = conn

	ch, err := conn.Channel()

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			logging.Logger.WithError(err).Info("could not open unique channel")
		}

		return err
	}

	channel = ch

	// In debug mode, log the infos
	if client.debug {
		logging.Logger.Info("connection to MQTT server successful")
	}

	return nil
}

func (client *mqttClient) Disconnect() error {
	err := connection.Close()

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			logging.Logger.WithError(err).Info("could not close the connection")
		}

		return err
	}

	return channel.Close()
}

// RetryMessage will ack an incoming AMQPMessage event and redeliver it if the maxRetry
// property is not exceeded
func (client *mqttClient) RetryMessage(event *AMQPMessage, maxRetry int) error {
	err := event.Ack(false)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			logging.Logger.WithFields(logging.LogFields{
				"messageId":  event.MessageId,
				"stacktrace": err,
			}).Info("could not acknowledge the event")
		}

		return err
	}

	redeliveredCount := event.IncrementRedeliveryHeader()

	// In debug mode, log the info
	if client.debug {
		logging.Logger.WithField("newRedeliveredCount", redeliveredCount).Info("incremented redelivered count")
	}

	if redeliveredCount <= maxRetry {
		// In debug mode, log the info
		if client.debug {
			logging.Logger.Info("redelivering event")
		}

		return client.redeliver(event)
	}

	return nil
}

// redeliver will resend an MQTT delivery from an acknowledged event
// the event will be sent to the same queue via the same exchanger with
// the same routing key. The event will also hold the same body and properties
func (client *mqttClient) redeliver(event *AMQPMessage) error {
	// Before sending a message, we need to make sure that Connection and Channel are valid
	if connection == nil || channel == nil {
		// In debug mode, log the warning
		if client.debug {
			logging.Logger.Warn("connection or channel is nil, attempting to reconnect")
		}

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			// In debug mode, log the error
			if client.debug {
				logging.Logger.WithError(err).Error("could not reconnect to rabbitMQ")
			}

			return err
		}
	}

	// Publish the message via the official amqp package
	// with our given configuration
	err := channel.Publish(
		event.Exchange,   // exchange
		event.RoutingKey, // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: event.ContentType,
			Body:        event.Body,
			Type:        event.RoutingKey,
			Priority:    event.Priority,
			MessageId:   event.MessageId,
			Headers:     event.Headers,
		},
	)

	// In debug mode, log the error
	if err != nil && client.debug {
		logging.Logger.WithError(err).Error("could not redeliver message")
	}

	return err
}

func NewClient(config ClientConfig) (MQTTClient, error) {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		debug:    false,
	}

	err := client.connect()

	if err != nil {
		// if maxRetry is set and greater than 0
		// we recursively call the constructor to
		// retry connecting
		if config.MaxRetry > 0 {
			config.MaxRetry -= 1
			time.Sleep(config.RetryDelay)

			// In debug mode, log the info
			if client.debug {
				logging.Logger.Info("retrying to connect to MQTT server")
			}

			return NewClient(config)
		}
		return nil, err
	}

	return client, nil
}

func NewClientDebug(config ClientConfig) (MQTTClient, error) {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		debug:    true,
	}

	err := client.connect()

	if err != nil {
		// if maxRetry is set and greater than 0
		// we recursively call the constructor to
		// retry connecting
		if config.MaxRetry > 0 {
			config.MaxRetry -= 1
			time.Sleep(config.RetryDelay)

			// In debug mode, log the info
			if client.debug {
				logging.Logger.Info("retrying to connect to MQTT server")
			}

			return NewClientDebug(config)
		}
		return nil, err
	}

	return client, nil
}
