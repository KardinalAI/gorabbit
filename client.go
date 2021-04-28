package gorabbit

import (
	"fmt"
	"github.com/streadway/amqp"
)

var (
	// Connection manages the serialization and deserialization of incoming
	// and outgoing frames and then dispatches them to the correct Channel
	Connection *amqp.Connection

	// Channel represents the AMQP channel
	Channel *amqp.Channel
)

type MQTTClient interface {
	connect() error
	SendEvent(exchange string, routingKey string, payload []byte) error
	SubscribeToEvents(queue string, consumer *string) (<-chan amqp.Delivery, error)
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
}

// SendEvent will send the desired payload through the selected channel
// exchange is the name of the exchange targeted for event publishing
// routingKey is the route that the exchange will use to forward the message
// payload is the object you want to send as a byte array
func (client *mqttClient) SendEvent(exchange string, routingKey string, payload []byte) error {
	// Before sending a message, we need to make sure that Connection and Channel are valid
	if Connection == nil || Channel == nil {

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			return err
		}
	}

	// Publish the message via the official amqp package
	// with our given configuration
	err := Channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        payload,
		},
	)

	return err
}

// SubscribeToEvents will connect to a queue and consume all incoming events from it
// queue is the name of the queue to connect to
// consumer[optional] is the unique identifier of the consumer. Leaving it empty will generate a unique identifier
// returns an incoming channel of amqp.Delivery (messages)
func (client *mqttClient) SubscribeToEvents(queue string, consumer *string) (<-chan amqp.Delivery, error) {
	// Before sending a message, we need to make sure that Connection and Channel are valid
	if Connection == nil || Channel == nil {

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			return nil, err
		}
	}

	// If the consumer is nil, replace it with an empty string
	randomConsumer := ""

	if consumer == nil {
		consumer = &randomConsumer
	}

	// Consume events via the official amqp package
	// with our given configuration
	messages, err := Channel.Consume(
		queue,     // queue
		*consumer, // consumer
		true,      // auto ack
		false,     // exclusive
		false,     // no local
		false,     // no wait
		nil,       // args
	)

	if err != nil {
		return nil, err
	}

	return messages, nil
}

// connect will initialize the AMQP connection, Connection and Channel
// given a configuration
func (client *mqttClient) connect() error {
	dialUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	conn, err := amqp.Dial(dialUrl)

	if err != nil {
		return err
	}

	Connection = conn

	ch, err := conn.Channel()

	if err != nil {
		return err
	}

	Channel = ch

	return nil
}

func NewMQTTClient(config ClientConfig) (MQTTClient, error) {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
	}

	err := client.connect()

	if err != nil {
		return nil, err
	}

	return client, nil
}
