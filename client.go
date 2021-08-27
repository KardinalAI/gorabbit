package gorabbit

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"time"
)

var (
	// Connection manages the serialization and deserialization of incoming
	// and outgoing frames and then dispatches them to the correct Channel
	connection *amqp.Connection

	// Channel represents the AMQP channel
	channel *amqp.Channel
)

type LogFields = map[string]interface{}

type MQTTClient interface {
	Disconnect() error
	NotifyClose() chan *amqp.Error
	SendMessage(exchange string, routingKey string, priority uint8, payload []byte) error
	RetryMessage(event *AMQPMessage, maxRetry int) error
	SubscribeToMessages(ctx context.Context, queue string, consumer string, autoAck bool) (<-chan AMQPMessage, error)
	CreateQueue(config QueueConfig) error
	CreateExchange(config ExchangeConfig) error
	BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error
	QueueIsEmpty(config QueueConfig) (bool, error)
	GetNumberOfMessages(config QueueConfig) (int, error)
	PopMessageFromQueue(queue string, autoAck bool) (*AMQPMessage, error)
	PurgeQueue(queue string) error
	DeleteQueue(queue string) error
	DeleteExchange(exchange string) error
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

	// logger used only in debug mode
	logger *logrus.Logger
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
			client.logger.Warn("connection or channel is nil, attempting to reconnect")
		}

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			// In debug mode, log the error
			if client.debug {
				client.logger.WithError(err).Error("could not reconnect to rabbitMQ")
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
		client.logger.WithError(err).Error("could not redeliver message")
	}

	return err
}

// SubscribeToMessages will connect to a queue and consume all incoming events from it
// Before an event is consumed, it will be parsed via ParseMessage method and then sent back for consumption
// queue is the name of the queue to connect to
// consumer[optional] is the unique identifier of the consumer. Leaving it empty will generate a unique identifier
// if autoAck is set to true, received events will be auto acknowledged as soon as they are consumed (received)
// returns an incoming channel of AMQPMessage (messages)
func (client *mqttClient) SubscribeToMessages(ctx context.Context, queue string, consumer string, autoAck bool) (<-chan AMQPMessage, error) {
	// Before sending a message, we need to make sure that Connection and Channel are valid
	if connection == nil || channel == nil {
		// In debug mode, log the warning
		if client.debug {
			client.logger.Warn("connection or channel is nil, attempting to reconnect")
		}

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			// In debug mode, log the error
			if client.debug {
				client.logger.WithError(err).Error("could not reconnect to rabbitMQ")
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
			client.logger.WithFields(LogFields{
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
		for {
			select {
			case <-ctx.Done():
				// In debug mode, log the event
				if client.debug {
					client.logger.WithFields(LogFields{
						"queue":    queue,
						"consumer": consumer,
					}).Error("message channel done")
				}
				return
			case message := <-messages:
				// In debug mode, log the event
				if client.debug {
					client.logger.WithFields(LogFields{
						"messageId":   message.MessageId,
						"deliverTag":  message.DeliveryTag,
						"redelivered": message.Redelivered,
					}).Info("received amqp delivery")
				}

				parsed, parseErr := ParseMessage(message)

				if parseErr == nil {
					// In debug mode, log the event
					if client.debug {
						client.logger.WithFields(LogFields{
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
						client.logger.WithError(parseErr).Error("could not parse AMQP message, sending delivery with empty properties")
					}

					parsedDeliveries <- AMQPMessage{
						Delivery: message,
					}
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
		client.logger.WithField("uri", dialUrl).Info("connecting to MQTT server")
	}

	conn, err := amqp.Dial(dialUrl)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("could not connect to MQTT server")
		}

		return err
	}

	connection = conn

	ch, err := conn.Channel()

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("could not open unique channel")
		}

		return err
	}

	err = ch.Qos(10, 0, false)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("Failed to set QoS")
		}

		return err
	}

	channel = ch

	// In debug mode, log the infos
	if client.debug {
		client.logger.Info("connection to MQTT server successful")
	}

	return nil
}

func (client *mqttClient) NotifyClose() chan *amqp.Error {
	return connection.NotifyClose(make(chan *amqp.Error))
}

func (client *mqttClient) Disconnect() error {
	err := connection.Close()

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("could not close the connection")
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
			client.logger.WithFields(LogFields{
				"messageId":  event.MessageId,
				"stacktrace": err,
			}).Info("could not acknowledge the event")
		}

		return err
	}

	redeliveredCount := event.IncrementRedeliveryHeader()

	// In debug mode, log the info
	if client.debug {
		client.logger.WithField("newRedeliveredCount", redeliveredCount).Info("incremented redelivered count")
	}

	if redeliveredCount <= maxRetry {
		// In debug mode, log the info
		if client.debug {
			client.logger.Info("redelivering event")
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
			client.logger.Warn("connection or channel is nil, attempting to reconnect")
		}

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			// In debug mode, log the error
			if client.debug {
				client.logger.WithError(err).Error("could not reconnect to rabbitMQ")
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
		client.logger.WithError(err).Error("could not redeliver message")
	}

	return err
}

// CreateQueue creates a new queue programmatically event though the MQTT
// server is already launched
func (client *mqttClient) CreateQueue(config QueueConfig) error {
	if channel == nil {
		return errors.New("mqtt channel is closed")
	}

	err := client.declareQueue(config)

	if err != nil {
		return err
	}

	if config.Bindings != nil {
		for _, binding := range *config.Bindings {
			err = client.addQueueBinding(config.Name, binding.RoutingKey, binding.Exchange)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateExchange creates a new exchange programmatically event though the MQTT
// server is already launched
func (client *mqttClient) CreateExchange(config ExchangeConfig) error {
	if channel == nil {
		return errors.New("mqtt channel is closed")
	}

	return client.declareExchange(config)
}

// BindExchangeToQueueViaRoutingKey binds an exchange to a queue via a given routingKey
//programmatically event though the MQTT server is already launched
func (client *mqttClient) BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error {
	if channel == nil {
		return errors.New("mqtt channel is closed")
	}

	return client.addQueueBinding(queue, routingKey, exchange)
}

// QueueIsEmpty returns an error if the queue doesn't exists,
// then check if it contains messages
func (client *mqttClient) QueueIsEmpty(config QueueConfig) (bool, error) {
	if channel == nil {
		return true, errors.New("mqtt channel is closed")
	}

	q, err := channel.QueueDeclarePassive(
		config.Name,
		config.Durable,
		false,
		config.Exclusive,
		false,
		nil,
	)

	if err != nil {
		return true, err
	}

	return q.Messages == 0, nil
}

// GetNumberOfMessages returns an error if the queue doesn't exists, and the number
// of messages if it does
func (client *mqttClient) GetNumberOfMessages(config QueueConfig) (int, error) {
	if channel == nil {
		return 0, errors.New("mqtt channel is closed")
	}

	q, err := channel.QueueDeclarePassive(
		config.Name,
		config.Durable,
		false,
		config.Exclusive,
		false,
		nil,
	)

	if err != nil {
		return 0, err
	}

	return q.Messages, nil
}

func (client *mqttClient) PopMessageFromQueue(queue string, autoAck bool) (*AMQPMessage, error) {
	if channel == nil {
		return nil, errors.New("mqtt channel is closed")
	}

	m, ok, err := channel.Get(queue, autoAck)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.New("queue is empty")
	}

	parsed, parseErr := ParseMessage(m)

	if parseErr != nil {
		return &AMQPMessage{
			Delivery: m,
		}, nil
	}

	return parsed, nil
}

// PurgeQueue will empty the given queue. An error is returned if the queue
// does not exist
func (client *mqttClient) PurgeQueue(queue string) error {
	if channel == nil {
		return errors.New("mqtt channel is closed")
	}

	_, err := channel.QueuePurge(queue, false)

	if err != nil {
		return err
	}

	return nil
}

// DeleteQueue will delete the given queue. An error is returned if the queue
// does not exist
func (client *mqttClient) DeleteQueue(queue string) error {
	if channel == nil {
		return errors.New("mqtt channel is closed")
	}

	_, err := channel.QueueDelete(queue, false, false, false)

	if err != nil {
		return err
	}

	return nil
}

// DeleteExchange will delete the given exchange. An error is returned if the exchange
// does not exist
func (client *mqttClient) DeleteExchange(exchange string) error {
	if channel == nil {
		return errors.New("mqtt channel is closed")
	}

	return channel.ExchangeDelete(exchange, false, false)
}

// declareExchange will initialize you exchange in the RabbitMQ server
func (client *mqttClient) declareExchange(config ExchangeConfig) error {
	err := channel.ExchangeDeclare(
		config.Name,       // name
		config.Type,       // type
		config.Persisted,  // durable
		!config.Persisted, // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)

	return err
}

// declareQueue will initialize you queue in the RabbitMQ server
func (client *mqttClient) declareQueue(config QueueConfig) error {
	_, err := channel.QueueDeclare(
		config.Name,      // name
		config.Durable,   // durable
		false,            // delete when unused
		config.Exclusive, // exclusive
		false,            // no-wait
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

// addQueueBinding will bind a queue to an exchange via a specific routing key
func (client *mqttClient) addQueueBinding(queue string, routingKey string, exchange string) error {
	err := channel.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)

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
				client.logger.Info("retrying to connect to MQTT server")
			}

			return NewClient(config)
		}
		return nil, err
	}

	return client, nil
}

func NewClientDebug(config ClientConfig, logger *logrus.Logger) (MQTTClient, error) {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		debug:    true,
		logger:   logger,
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
				logger.Info("retrying to connect to MQTT server")
			}

			return NewClientDebug(config, logger)
		}
		return nil, err
	}

	return client, nil
}
