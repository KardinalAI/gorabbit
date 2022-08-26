package gorabbit

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
	"time"
)

var (
	consumed *ttlMap
)

const (
	cacheTTL = 8 * time.Second

	cacheLimit = 1024

	reconnectDelay = 3 * time.Second
)

type LogFields = map[string]interface{}

type MQTTClient interface {
	Disconnect() error
	ListenStatus() <-chan ConnectionStatus
	SendMessage(exchange string, routingKey string, priority MessagePriority, payload []byte) error
	RetryMessage(event *AMQPMessage, maxRetry int) error
	SubscribeToMessages(queue string, consumer string, autoAck bool) (<-chan AMQPMessage, error)
	CreateQueue(config QueueConfig) error
	CreateExchange(config ExchangeConfig) error
	BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error
	GetNumberOfMessages(config QueueConfig) (int, error)
	PopMessageFromQueue(queue string, autoAck bool) (*AMQPMessage, error)
	PurgeQueue(queue string) error
	DeleteQueue(queue string) error
	DeleteExchange(exchange string) error
	ReadyCheck() bool
}

type mqttClient struct {
	// Host is the domain name of the RabbitMQ server
	Host string

	// Port is the configured RabbitMQ port, default usually is 5672
	Port uint

	// Username is the username used when setting up RabbitMQ
	Username string

	// Password is the password used when setting up RabbitMQ
	Password string

	// logger used only in debug mode
	logger Logger

	// connectionManager manages the connection and channel logic and high-level logic
	// such as keep alive mechanism and health check
	connectionManager *connectionManager

	ctx context.Context

	cancel context.CancelFunc
}

func NewClient(config ClientConfig) MQTTClient {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
	}

	mode := config.Mode
	if mode == "" {
		mode = os.Getenv("GORABBIT_MODE")
	}

	switch mode {
	case Debug:
		client.logger = &stdLogger{}
	default:
		client.logger = &noLogger{}
	}

	client.ctx, client.cancel = context.WithCancel(context.Background())

	dialUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	client.connectionManager = newManager(client.ctx, dialUrl, config.KeepAlive, config.OnConnectionStatusChanged, client.logger)

	if consumed == nil {
		consumed = newTTLMap(cacheLimit, cacheTTL)
	}

	return client
}

// Deprecated: Use NewClient instead and set the corresponding ClientConfig.Mode property
// or "GORABBIT_MODE" environment variable.
func NewClientDebug(config ClientConfig) MQTTClient {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		logger:   &stdLogger{},
	}

	client.ctx, client.cancel = context.WithCancel(context.Background())

	dialUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	client.logger.Printf("Connection to MQTT server with url: %s", dialUrl)

	client.connectionManager = newManager(client.ctx, dialUrl, config.KeepAlive, config.OnConnectionStatusChanged, client.logger)

	if consumed == nil {
		consumed = newTTLMap(cacheLimit, cacheTTL)
	}

	return client
}

// SendMessage will send the desired payload through the selected channel
// exchange is the name of the exchange targeted for event publishing
// routingKey is the route that the exchange will use to forward the message
// priority is the priority level of the message (1 to 7)
// payload is the object you want to send as a byte array
func (client *mqttClient) SendMessage(exchange string, routingKey string, priority MessagePriority, payload []byte) error {
	// Publish the message via the official amqp package
	// with our given configuration
	err := client.connectionManager.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        payload,
			Type:        routingKey,
			Priority:    priority.Uint8(),
			MessageId:   uuid.NewString(),
			Headers: map[string]interface{}{
				RedeliveryHeader: 0,
			},
			Timestamp: time.Now(),
		},
	)

	// log the error
	if err != nil {
		client.logger.Printf("Could not send message: %s", err.Error())
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
	// Consume events via the official amqp package
	// with our given configuration
	messages, err := client.connectionManager.Consume(
		queue,    // queue
		consumer, // consumer
		autoAck,  // auto ack
		false,    // exclusive
		false,    // no local
		false,    // no wait
		nil,      // args
	)

	if err != nil {
		client.logger.Printf("could not consume rabbitMQ messages from queue %s", queue)

		return nil, err
	}

	parsedDeliveries := make(chan AMQPMessage)

	go func() {
		for message := range messages {
			client.logger.Printf("Received amqp delivery with tag %d and id %s", message.DeliveryTag, message.MessageId)

			parsed, parseErr := ParseMessage(message)

			if parseErr == nil {
				client.logger.Printf("AMQP message successfully parsed and sent")

				parsedDeliveries <- *parsed
			} else {
				client.logger.Printf("could not parse AMQP message, sending native delivery")

				parsedDeliveries <- AMQPMessage{
					Delivery: message,
				}
			}
		}
	}()

	return parsedDeliveries, nil
}

func (client *mqttClient) ListenStatus() <-chan ConnectionStatus {
	return client.connectionManager.connectionStatus
}

func (client *mqttClient) Disconnect() error {
	err := client.connectionManager.close()

	if err != nil {
		client.logger.Printf("Could not disconnect: %s", err.Error())

		return err
	}

	// cancel the context to stop all reconnection goroutines
	client.cancel()
	return nil
}

// RetryMessage will ack an incoming AMQPMessage event and redeliver it if the maxRetry
// property is not exceeded
func (client *mqttClient) RetryMessage(event *AMQPMessage, maxRetry int) error {
	err := event.Ack(false)

	if err != nil {
		client.logger.Printf("Could not acknowledge message %s", event.MessageId)

		return err
	}

	redeliveredCount := event.IncrementRedeliveryHeader()

	client.logger.Printf("Incremented redelivered count to %d", redeliveredCount)

	if redeliveredCount <= maxRetry {
		client.logger.Printf("Redelivering message")

		return client.connectionManager.Publish(
			event.Exchange,
			event.RoutingKey,
			false,
			false,
			event.ToPublishing(),
		)
	}

	return errors.New("max retry has been reached")
}

// CreateQueue creates a new queue programmatically event though the MQTT
// server is already launched
func (client *mqttClient) CreateQueue(config QueueConfig) error {
	_, err := client.connectionManager.QueueDeclare(
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

	if config.Bindings != nil {
		for _, binding := range *config.Bindings {
			err = client.BindExchangeToQueueViaRoutingKey(binding.Exchange, config.Name, binding.RoutingKey)

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
	return client.connectionManager.ExchangeDeclare(
		config.Name,       // name
		config.Type,       // type
		config.Persisted,  // durable
		!config.Persisted, // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
}

// BindExchangeToQueueViaRoutingKey binds an exchange to a queue via a given routingKey
//programmatically event though the MQTT server is already launched
func (client *mqttClient) BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error {
	return client.connectionManager.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)
}

// GetNumberOfMessages returns an error if the queue doesn't exists, and the number
// of messages if it does
func (client *mqttClient) GetNumberOfMessages(config QueueConfig) (int, error) {
	q, err := client.connectionManager.QueueDeclarePassive(
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

// PopMessageFromQueue fetches the latest message in queue if the queue is not empty.
// If autoAck is true, the message will automatically be acknowledged once popped
func (client *mqttClient) PopMessageFromQueue(queue string, autoAck bool) (*AMQPMessage, error) {
	m, ok, err := client.connectionManager.Get(queue, autoAck)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.New("queue is empty")
	}

	consumed.Put(m.DeliveryTag)

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
	_, err := client.connectionManager.QueuePurge(queue, false)

	if err != nil {
		return err
	}

	return nil
}

// DeleteQueue will delete the given queue. An error is returned if the queue
// does not exist
func (client *mqttClient) DeleteQueue(queue string) error {
	_, err := client.connectionManager.QueueDelete(queue, false, false, false)

	if err != nil {
		return err
	}

	return nil
}

// DeleteExchange will delete the given exchange. An error is returned if the exchange
// does not exist
func (client *mqttClient) DeleteExchange(exchange string) error {
	return client.connectionManager.ExchangeDelete(exchange, false, false)
}

// ReadyCheck returns true if the connection to MQTT server is established successfully, and
// all subscriptions are healthy (running successfully)
func (client *mqttClient) ReadyCheck() bool {
	return client.connectionManager.isOperational() && client.connectionManager.isHealthy()
}
