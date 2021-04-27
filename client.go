package go_mqtt

import (
	"fmt"
	"github.com/streadway/amqp"
)

var (
	Connection *amqp.Connection
	Channel    *amqp.Channel
)

type MQTTClient interface {
	connect() error
	SendEvent(exchange string, routingKey string, payload []byte) error
	SubscribeToEvents(queue string, consumer *string) (<-chan amqp.Delivery, error)
}

type mqttClient struct {
	Host     string
	Port     uint
	Username string
	Password string
}

func (client *mqttClient) SendEvent(exchange string, routingKey string, payload []byte) error {
	if Connection == nil || Channel == nil {
		err := client.connect()

		if err != nil {
			return err
		}
	}

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

func (client *mqttClient) SubscribeToEvents(queue string, consumer *string) (<-chan amqp.Delivery, error) {
	if Connection == nil || Channel == nil {
		err := client.connect()

		if err != nil {
			return nil, err
		}
	}

	randomConsumer := ""

	if consumer == nil {
		consumer = &randomConsumer
	}

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
