package gorabbit

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

var consumed *ttlMap

const (
	cacheTTL   = 8 * time.Second
	cacheLimit = 1024
)

type MQTTClient interface {
	// Disconnect launches the disconnection process.
	Disconnect() error

	// SendMessage will send the desired payload through the selected channel.
	//  - exchange is the name of the exchange targeted for event publishing.
	//  - routingKey is the route that the exchange will use to forward the message.
	//  - payload is the object you want to send as a byte array.
	// Optionally you can add sendOptions for extra customization.
	SendMessage(exchange string, routingKey string, payload []byte, options ...*sendOptions) error

	// RetryMessage will ack an incoming AMQPMessage event and redeliver it if the maxRetry property is not exceeded.
	RetryMessage(event *AMQPMessage, maxRetry int) error

	// SubscribeToMessages will connect to a queue and consume all incoming events from it.
	// Before an event is consumed, it will be parsed via ParseMessage method and then sent back for consumption
	//  - queue is the name of the queue to connect to.
	//  - consumer[optional] is the unique identifier of the consumer. Leaving it empty will generate a unique identifier.
	//  - if autoAck is set to true, received events will be auto acknowledged as soon as they are consumed (received).
	// returns an incoming channel of AMQPMessage (messages).
	SubscribeToMessages(queue string, consumer string, autoAck bool) (<-chan AMQPMessage, error)

	// Editor
	CreateQueue(config QueueConfig) error
	CreateExchange(config ExchangeConfig) error
	BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error
	GetNumberOfMessages(config QueueConfig) (int, error)
	PopMessageFromQueue(queue string, autoAck bool) (*AMQPMessage, error)
	PurgeQueue(queue string) error
	DeleteQueue(queue string) error
	DeleteExchange(exchange string) error

	// Health
	ReadyCheck() bool
}

type mqttClient struct {
	// Host is the RabbitMQ server host name.
	Host string

	// Port is the RabbitMQ server port number.
	Port uint

	// Username is the RabbitMQ server allowed username.
	Username string

	// Password is the RabbitMQ server allowed password.
	Password string

	// logger defines the logger used, depending on the mode set.
	logger Logger

	// disabled completely disables the client if true.
	disabled bool

	// connectionManager manages the connection and channel logic and high-level logic
	// such as keep alive mechanism and health check.
	connectionManager *connectionManager

	ctx context.Context

	cancel context.CancelFunc
}

// NewClient will instantiate a new MQTTClient.
// If options is set to nil, the DefaultClientOptions will be used.
// Optionally, listeners can be passed for extra high-level functionalities, or simple logs with DefaultListeners.
func NewClient(options *clientOptions, listeners ...*ClientListeners) MQTTClient {
	// If no options is passed, we use the DefaultClientOptions.
	if options == nil {
		options = DefaultClientOptions()
	}

	client := &mqttClient{
		Host:     options.host,
		Port:     options.port,
		Username: options.username,
		Password: options.password,
	}

	// We check if the disabled flag is present, which will completely disable the MQTTClient.
	if disabledOverride := os.Getenv("GORABBIT_DISABLED"); disabledOverride != "" {
		isDisabled := disabledOverride == "1" || disabledOverride == "true"

		if isDisabled {
			client.disabled = true

			return client
		}
	}

	// We check if the mode was overwritten with the environment variable "GORABBIT_MODE".
	if modeOverride := os.Getenv("GORABBIT_MODE"); modeOverride != "" && isValidMode(modeOverride) {
		// We override the mode only if it is valid
		options.mode = modeOverride
	}

	switch options.mode {
	case Debug:
		// If the mode is Debug, we want to actually log important events.
		client.logger = &stdLogger{}
	default:
		// Otherwise, we do not want any logs coming from the library.
		client.logger = &noLogger{}
	}

	client.ctx, client.cancel = context.WithCancel(context.Background())

	dialURL := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	var statusListeners *ClientListeners

	if listeners != nil && listeners[0] != nil {
		statusListeners = listeners[0]
	}

	client.connectionManager = newManager(client.ctx, dialURL, options.keepAlive, options.retryDelay, options.maxRetry, statusListeners, client.logger)

	// If the consumed cache is not present, we instantiate it.
	if consumed == nil {
		consumed = newTTLMap(cacheLimit, cacheTTL)
	}

	return client
}

func (client *mqttClient) SendMessage(exchange string, routingKey string, payload []byte, options ...*sendOptions) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

	publishing := &amqp.Publishing{
		ContentType:  "text/plain",
		Body:         payload,
		Type:         routingKey,
		Priority:     PriorityMedium.Uint8(),
		DeliveryMode: Persistent.Uint8(),
		MessageId:    uuid.NewString(),
		Headers: map[string]interface{}{
			RedeliveryHeader: 0,
		},
		Timestamp: time.Now(),
	}

	if options != nil && options[0] != nil {
		opts := options[0]

		publishing.Priority = opts.priority()
		publishing.DeliveryMode = opts.mode()
	}

	// Publish the message via the official amqp package
	// with our given configuration
	err := client.connectionManager.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		*publishing,
	)

	// log the error
	if err != nil {
		client.logger.Printf("Could not send message: %s", err.Error())
	}

	return err
}

func (client *mqttClient) SubscribeToMessages(queue string, consumer string, autoAck bool) (<-chan AMQPMessage, error) {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		// nolint: nilnil // We must return <nil, nil>
		return nil, nil
	}

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

func (client *mqttClient) Disconnect() error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

	err := client.connectionManager.close()

	if err != nil {
		client.logger.Printf("Could not disconnect: %s", err.Error())

		return err
	}

	// cancel the context to stop all reconnection goroutines
	client.cancel()

	return nil
}

func (client *mqttClient) RetryMessage(event *AMQPMessage, maxRetry int) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

	if err := event.Ack(false); err != nil {
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

	return errMaxRetryReached
}

// CreateQueue creates a new queue programmatically event though the MQTT
// server is already launched.
func (client *mqttClient) CreateQueue(config QueueConfig) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

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
// server is already launched.
func (client *mqttClient) CreateExchange(config ExchangeConfig) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

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

func (client *mqttClient) BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

	return client.connectionManager.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)
}

// GetNumberOfMessages returns an error if the queue doesn't exist, and the number
// of messages if it does.
func (client *mqttClient) GetNumberOfMessages(config QueueConfig) (int, error) {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return -1, nil
	}

	q, err := client.connectionManager.QueueDeclarePassive(
		config.Name,
		config.Durable,
		false,
		config.Exclusive,
		false,
		nil,
	)

	if err != nil {
		return -1, err
	}

	return q.Messages, nil
}

// PopMessageFromQueue fetches the latest message in queue if the queue is not empty.
// If autoAck is true, the message will automatically be acknowledged once popped.
func (client *mqttClient) PopMessageFromQueue(queue string, autoAck bool) (*AMQPMessage, error) {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		// nolint: nilnil // We must return <nil, nil>
		return nil, nil
	}

	m, ok, err := client.connectionManager.Get(queue, autoAck)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errEmptyQueue
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
// does not exist.
func (client *mqttClient) PurgeQueue(queue string) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

	_, err := client.connectionManager.QueuePurge(queue, false)

	if err != nil {
		return err
	}

	return nil
}

// DeleteQueue will delete the given queue. An error is returned if the queue
// does not exist.
func (client *mqttClient) DeleteQueue(queue string) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

	_, err := client.connectionManager.QueueDelete(queue, false, false, false)

	if err != nil {
		return err
	}

	return nil
}

// DeleteExchange will delete the given exchange. An error is returned if the exchange
// does not exist.
func (client *mqttClient) DeleteExchange(exchange string) error {
	// client is disabled, so we do nothing and return no error
	if client.disabled {
		return nil
	}

	return client.connectionManager.ExchangeDelete(exchange, false, false)
}

// ReadyCheck returns true if the connection to MQTT server is established successfully, and
// all subscriptions are healthy (running successfully).
func (client *mqttClient) ReadyCheck() bool {
	// client is disabled, so we do nothing and return true
	if client.disabled {
		return true
	}

	return client.connectionManager.isOperational() && client.connectionManager.isHealthy()
}
