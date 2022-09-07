package gorabbit

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

var consumed *ttlMap[uint64, interface{}]

type MQTTClient interface {
	// Disconnect launches the disconnection process.
	// This operation disables to client permanently.
	Disconnect() error

	// SendMessage will send the desired payload through the selected channel.
	//  - exchange is the name of the exchange targeted for event publishing.
	//  - routingKey is the route that the exchange will use to forward the message.
	//  - payload is the object you want to send as a byte array.
	// Returns an error if the connection to the RabbitMQ server is down.
	SendMessage(exchange, routingKey string, payload []byte) error

	// SendMessageWithOptions will send the desired payload through the selected channel.
	//  - exchange is the name of the exchange targeted for event publishing.
	//  - routingKey is the route that the exchange will use to forward the message.
	//  - payload is the object you want to send as a byte array.
	// Optionally you can add sendOptions for extra customization.
	// Returns an error if the connection to the RabbitMQ server is down.
	SendMessageWithOptions(exchange, routingKey string, payload []byte, options *sendOptions) error

	// SubscribeToMessages will connect to a queue and consume all incoming events from it.
	// Before an event is consumed, it will be parsed via parseMessage method and then sent back for consumption
	//  - queue is the name of the queue to connect to.
	//  - consumer[optional] is the unique identifier of the consumer. Leaving it empty will generate a unique identifier.
	//  - if autoAck is set to true, received events will be auto acknowledged as soon as they are consumed (received).
	// returns an incoming channel of amqpMessage (messages).
	SubscribeToMessages(queue string, consumer string, autoAck bool) (<-chan amqpMessage, error)

	// RegisterConsumer will register a MessageConsumer for internal queue subscription and message processing.
	// The MessageConsumer will hold a list of MQTTMessageHandlers to internalize message processing.
	// Based on the return of error of each handler, the process of acknowledgment, rejection and retry of messages is
	// fully handled internally.
	// Furthermore, connection lost and channel errors are also internally handled by the connectionManager that will keep consumers
	// alive if and when necessary.
	RegisterConsumer(consumer MessageConsumer)

	// CreateQueue will create a new queue from QueueConfig.
	CreateQueue(config QueueConfig) error

	// CreateExchange will create a new exchange from ExchangeConfig.
	CreateExchange(config ExchangeConfig) error

	// BindExchangeToQueueViaRoutingKey will bind an exchange to a queue via a given routingKey.
	// Returns an error if the connection to the RabbitMQ server is down or if the exchange or queue does not exist.
	BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error

	// GetNumberOfMessages retrieves the number of messages currently sitting in a given queue.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	GetNumberOfMessages(config QueueConfig) (int, error)

	// PopMessageFromQueue retrieves the first message of a queue. The message can then be auto-acknowledged or not.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist or is empty.
	PopMessageFromQueue(queue string, autoAck bool) (*amqpMessage, error)

	// PurgeQueue will empty a queue of all its current messages.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	PurgeQueue(queue string) error

	// DeleteQueue permanently deletes an existing queue.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	DeleteQueue(queue string) error

	// DeleteExchange permanently deletes an existing exchange.
	// Returns an error if the connection to the RabbitMQ server is down or the exchange does not exist.
	DeleteExchange(exchange string) error

	// ReadyCheck returns true if the client is fully operational, connected to the RabbitMQ and have all its subscriptionsHealth up.
	// Returns false if one of the above failed.
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
func NewClient(options *clientOptions) MQTTClient {
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

	client.connectionManager = newManager(
		client.ctx,
		dialURL,
		options.keepAlive,
		options.retryDelay,
		options.maxRetry,
		options.publishingCacheSize,
		options.publishingCacheTTL,
		client.logger,
	)

	// If the consumed cache is not present, we instantiate it.
	if consumed == nil {
		consumed = newTTLMap[uint64, interface{}](options.consumedCacheSize, options.consumedCacheTTL)
	}

	return client
}

func (client *mqttClient) SendMessage(exchange string, routingKey string, payload []byte) error {
	// client is disabled, so we do nothing and return no error.
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

	// Publish the message via the official amqp package with our given configuration.
	err := client.connectionManager.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		*publishing,
		false,
	)

	// log the error
	if err != nil {
		client.logger.Printf("Could not send message: %s", err.Error())
	}

	return err
}

func (client *mqttClient) SendMessageWithOptions(exchange string, routingKey string, payload []byte, options *sendOptions) error {
	// client is disabled, so we do nothing and return no error.
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

	if options != nil {
		publishing.Priority = options.priority()
		publishing.DeliveryMode = options.mode()
	}

	// Publish the message via the official amqp package with our given configuration.
	err := client.connectionManager.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		*publishing,
		false,
	)

	// log the error
	if err != nil {
		client.logger.Printf("Could not send message: %s", err.Error())
	}

	return err
}

func (client *mqttClient) SubscribeToMessages(queue string, consumer string, autoAck bool) (<-chan amqpMessage, error) {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		// nolint: nilnil // We must return <nil, nil>
		return nil, nil
	}

	// RegisterConsumer events via the official amqp package with our given configuration.
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

	parsedDeliveries := make(chan amqpMessage)

	go func() {
		for message := range messages {
			client.logger.Printf("Received amqp delivery with tag %d and id %s", message.DeliveryTag, message.MessageId)

			parsed, parseErr := ParseMessage(message)

			if parseErr == nil {
				client.logger.Printf("AMQP message successfully parsed and sent")

				parsedDeliveries <- *parsed
			} else {
				client.logger.Printf("could not parse AMQP message, sending native delivery")

				parsedDeliveries <- amqpMessage{
					Delivery: message,
				}
			}
		}
	}()

	return parsedDeliveries, nil
}

func (client *mqttClient) RegisterConsumer(consumer MessageConsumer) {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return
	}

	client.connectionManager.registerConsumer(consumer)
}

func (client *mqttClient) Disconnect() error {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return nil
	}

	err := client.connectionManager.close()

	if err != nil {
		client.logger.Printf("Could not disconnect: %s", err.Error())

		return err
	}

	// cancel the context to stop all reconnection goroutines.
	client.cancel()

	// disable the client to avoid trying to launch new operations.
	client.disabled = true

	return nil
}

func (client *mqttClient) CreateQueue(config QueueConfig) error {
	// client is disabled, so we do nothing and return no error.
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

func (client *mqttClient) CreateExchange(config ExchangeConfig) error {
	// client is disabled, so we do nothing and return no error.
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

func (client *mqttClient) BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error {
	// client is disabled, so we do nothing and return no error.
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

func (client *mqttClient) GetNumberOfMessages(config QueueConfig) (int, error) {
	// client is disabled, so we do nothing and return no error.
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

func (client *mqttClient) PopMessageFromQueue(queue string, autoAck bool) (*amqpMessage, error) {
	// client is disabled, so we do nothing and return no error.
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

	consumed.Put(m.DeliveryTag, nil)

	parsed, parseErr := ParseMessage(m)

	if parseErr != nil {
		return &amqpMessage{
			Delivery: m,
		}, nil
	}

	return parsed, nil
}

func (client *mqttClient) PurgeQueue(queue string) error {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return nil
	}

	_, err := client.connectionManager.QueuePurge(queue, false)

	if err != nil {
		return err
	}

	return nil
}

func (client *mqttClient) DeleteQueue(queue string) error {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return nil
	}

	_, err := client.connectionManager.QueueDelete(queue, false, false, false)

	if err != nil {
		return err
	}

	return nil
}

func (client *mqttClient) DeleteExchange(exchange string) error {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return nil
	}

	return client.connectionManager.ExchangeDelete(exchange, false, false)
}

func (client *mqttClient) ReadyCheck() bool {
	// client is disabled, so we do nothing and return true.
	if client.disabled {
		return true
	}

	return client.connectionManager.isOperational() && client.connectionManager.isHealthy()
}
