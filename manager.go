package gorabbit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// MQTTManager is a simple MQTT interface that offers basic management operations such as:
// 	 - Creation of queue, exchange and bindings
//   - Deletion of queues and exchanges
//   - Purge of queues
//   - Queue evaluation (existence and number of messages)
type MQTTManager interface {
	// Disconnect launches the disconnection process.
	// This operation disables to manager permanently.
	Disconnect() error

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

	// PushMessageToExchange pushes a message to a given exchange with a given routing key.
	// Returns an error if the connection to the RabbitMQ server is down or the exchange does not exist.
	PushMessageToExchange(exchange, routingKey string, payload interface{}) error

	// PopMessageFromQueue retrieves the first message of a queue. The message can then be auto-acknowledged or not.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist or is empty.
	PopMessageFromQueue(queue string, autoAck bool) (*amqp.Delivery, error)

	// PurgeQueue will empty a queue of all its current messages.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	PurgeQueue(queue string) error

	// DeleteQueue permanently deletes an existing queue.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	DeleteQueue(queue string) error

	// DeleteExchange permanently deletes an existing exchange.
	// Returns an error if the connection to the RabbitMQ server is down or the exchange does not exist.
	DeleteExchange(exchange string) error
}

type mqttManager struct {
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

	// connection holds the single connection to the RabbitMQ server.
	connection *amqp.Connection

	// channel holds the single channel from the connection.
	channel *amqp.Channel
}

// NewManager will instantiate a new MQTTManager.
// If options is set to nil, the DefaultManagerOptions will be used.
func NewManager(options *ManagerOptions) (MQTTManager, error) {
	// If no options is passed, we use the DefaultClientOptions.
	if options == nil {
		options = DefaultManagerOptions()
	}

	client := &mqttManager{
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

			return client, nil
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

	dialURL := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	var err error

	client.connection, err = amqp.Dial(dialURL)
	if err != nil {
		return client, err
	}

	client.channel, err = client.connection.Channel()
	if err != nil {
		return client, err
	}

	return client, nil
}

func (manager *mqttManager) Disconnect() error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// We close the manager's channel only if it is opened.
	if manager.channel != nil && !manager.channel.IsClosed() {
		err := manager.channel.Close()
		if err != nil {
			return err
		}
	}

	// We close the manager's connection only if it is opened.
	if manager.connection != nil && !manager.connection.IsClosed() {
		return manager.connection.Close()
	}

	return nil
}

func (manager *mqttManager) CreateQueue(config QueueConfig) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We declare the queue via the channel.
	_, err := manager.channel.QueueDeclare(
		config.Name,      // name
		config.Durable,   // durable
		false,            // delete when unused
		config.Exclusive, // exclusive
		false,            // no-wait
		config.Args,
	)

	if err != nil {
		return err
	}

	// If bindings are also declared, we create the bindings too.
	if config.Bindings != nil {
		for _, binding := range *config.Bindings {
			err = manager.BindExchangeToQueueViaRoutingKey(binding.Exchange, config.Name, binding.RoutingKey)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (manager *mqttManager) CreateExchange(config ExchangeConfig) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We declare the exchange via the channel.
	return manager.channel.ExchangeDeclare(
		config.Name,          // name
		config.Type.String(), // type
		config.Persisted,     // durable
		!config.Persisted,    // auto-deleted
		false,                // internal
		false,                // no-wait
		config.Args,          // arguments
	)
}

func (manager *mqttManager) BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We bind the queue to a given exchange and routing key via the channel.
	return manager.channel.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)
}

func (manager *mqttManager) GetNumberOfMessages(config QueueConfig) (int, error) {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return -1, nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return -1, err
	}

	// We passively declare the queue via the channel, this will return the existing queue or an error if it doesn't exist.
	q, err := manager.channel.QueueDeclarePassive(
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

func (manager *mqttManager) PushMessageToExchange(exchange, routingKey string, payload interface{}) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We convert the payload to a []byte.
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// We build the amqp.Publishing object.
	publishing := amqp.Publishing{
		ContentType:  "text/plain",
		Body:         payloadBytes,
		Type:         routingKey,
		Priority:     PriorityMedium.Uint8(),
		DeliveryMode: Transient.Uint8(),
		MessageId:    uuid.NewString(),
		Timestamp:    time.Now(),
	}

	// We push the message via the channel.
	return manager.channel.PublishWithContext(context.TODO(), exchange, routingKey, false, false, publishing)
}

func (manager *mqttManager) PopMessageFromQueue(queue string, autoAck bool) (*amqp.Delivery, error) {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		// nolint: nilnil // We must return <nil, nil>
		return nil, nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return nil, err
	}

	// We get the message via the channel.
	m, ok, err := manager.channel.Get(queue, autoAck)

	if err != nil {
		return nil, err
	}

	// If the queue is empty.
	if !ok {
		return nil, errEmptyQueue
	}

	return &m, nil
}

func (manager *mqttManager) PurgeQueue(queue string) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We purge the queue via the channel.
	_, err := manager.channel.QueuePurge(queue, false)

	if err != nil {
		return err
	}

	return nil
}

func (manager *mqttManager) DeleteQueue(queue string) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We delete the queue via the channel.
	_, err := manager.channel.QueueDelete(queue, false, false, false)

	if err != nil {
		return err
	}

	return nil
}

func (manager *mqttManager) DeleteExchange(exchange string) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We delete the exchange via the channel.
	return manager.channel.ExchangeDelete(exchange, false, false)
}

func (manager *mqttManager) ready() (bool, error) {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return true, nil
	}

	// If the connection is nil or closed, we return an error because the manager is not ready.
	if manager.connection == nil || manager.connection.IsClosed() {
		return false, errConnectionClosed
	}

	// If the channel is nil or closed, we return an error because the manager is not ready.
	if manager.channel == nil || manager.channel.IsClosed() {
		return false, errChannelClosed
	}

	return true, nil
}
