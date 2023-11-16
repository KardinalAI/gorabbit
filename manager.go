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
//   - Creation of queue, exchange and bindings
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
	GetNumberOfMessages(queue string) (int, error)

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

	// SetupFromDefinitions loads a definitions.json file and automatically sets up exchanges, queues and bindings.
	SetupFromDefinitions(path string) error

	// GetHost returns the host used to initialize the manager.
	GetHost() string

	// GetPort returns the port used to initialize the manager.
	GetPort() uint

	// GetUsername returns the username used to initialize the manager.
	GetUsername() string

	// GetVhost returns the vhost used to initialize the manager.
	GetVhost() string

	// IsDisabled returns whether the manager is disabled or not.
	IsDisabled() bool
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

	// Vhost is used for CloudAMQP connections to set the specific vhost.
	Vhost string

	// logger defines the logger used, depending on the mode set.
	logger logger

	// disabled completely disables the manager if true.
	disabled bool

	// connection holds the single connection to the RabbitMQ server.
	connection *amqp.Connection

	// channel holds the single channel from the connection.
	channel *amqp.Channel
}

// NewManager will instantiate a new MQTTManager.
// If options is set to nil, the DefaultManagerOptions will be used.
func NewManager(options *ManagerOptions) (MQTTManager, error) {
	// If no options is passed, we use the DefaultManagerOptions.
	if options == nil {
		options = DefaultManagerOptions()
	}

	return newManagerFromOptions(options)
}

// NewManagerFromEnv will instantiate a new MQTTManager from environment variables.
func NewManagerFromEnv() (MQTTManager, error) {
	options := NewManagerOptionsFromEnv()

	return newManagerFromOptions(options)
}

func newManagerFromOptions(options *ManagerOptions) (MQTTManager, error) {
	manager := &mqttManager{
		Host:     options.Host,
		Port:     options.Port,
		Username: options.Username,
		Password: options.Password,
		Vhost:    options.Vhost,
		logger:   &noLogger{},
	}

	// We check if the disabled flag is present, which will completely disable the MQTTManager.
	if disabledOverride := os.Getenv("GORABBIT_DISABLED"); disabledOverride != "" {
		switch disabledOverride {
		case "1", "true":
			manager.disabled = true
			return manager, nil
		}
	}

	// We check if the mode was overwritten with the environment variable "GORABBIT_MODE".
	if modeOverride := os.Getenv("GORABBIT_MODE"); isValidMode(modeOverride) {
		// We override the mode only if it is valid
		options.Mode = modeOverride
	}

	if options.Mode == Debug {
		// If the mode is Debug, we want to actually log important events.
		manager.logger = newStdLogger()
	}

	protocol := defaultProtocol

	if options.UseTLS {
		protocol = securedProtocol
	}

	dialURL := fmt.Sprintf("%s://%s:%s@%s:%d/%s", protocol, manager.Username, manager.Password, manager.Host, manager.Port, manager.Vhost)

	var err error

	manager.connection, err = amqp.Dial(dialURL)
	if err != nil {
		return manager, err
	}

	manager.channel, err = manager.connection.Channel()
	if err != nil {
		return manager, err
	}

	return manager, nil
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
		for _, binding := range config.Bindings {
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

func (manager *mqttManager) GetNumberOfMessages(queue string) (int, error) {
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
		queue,
		false,
		false,
		false,
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
		ContentType:  "application/json",
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
		//nolint: nilnil // We must return <nil, nil>
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

func (manager *mqttManager) SetupFromDefinitions(path string) error {
	// Manager is disabled, so we do nothing and return no error.
	if manager.disabled {
		return nil
	}

	// If the manager is not ready, we return its error.
	if ready, err := manager.ready(); !ready {
		return err
	}

	// We read the definitions.json file.
	definitions, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	def := new(SchemaDefinitions)

	// We parse the definitions.json file into the corresponding struct.
	err = json.Unmarshal(definitions, def)
	if err != nil {
		return err
	}

	for _, queue := range def.Queues {
		// We create the queue.
		err = manager.CreateQueue(QueueConfig{
			Name:      queue.Name,
			Durable:   queue.Durable,
			Exclusive: false,
		})

		if err != nil {
			return err
		}
	}

	for _, exchange := range def.Exchanges {
		// We create the exchange.
		err = manager.CreateExchange(ExchangeConfig{
			Name:      exchange.Name,
			Type:      ExchangeType(exchange.Type),
			Persisted: exchange.Durable,
		})

		if err != nil {
			return err
		}
	}

	for _, binding := range def.Bindings {
		// We bind the given exchange to the given queue via the given routing key.
		err = manager.BindExchangeToQueueViaRoutingKey(binding.Source, binding.Destination, binding.RoutingKey)

		if err != nil {
			return err
		}
	}

	return nil
}

func (manager *mqttManager) checkChannel() error {
	var err error

	// If the connection is nil or closed, we must request a new channel.
	if manager.channel == nil || manager.channel.IsClosed() {
		manager.channel, err = manager.connection.Channel()
	}

	return err
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

	// We check the channel as it might have been closed, and we need the request a new one.
	if err := manager.checkChannel(); err != nil {
		return false, err
	}

	// If the channel is still nil or closed, we return an error because the manager is not ready.
	if manager.channel == nil || manager.channel.IsClosed() {
		return false, errChannelClosed
	}

	return true, nil
}

func (manager *mqttManager) GetHost() string {
	return manager.Host
}

func (manager *mqttManager) GetPort() uint {
	return manager.Port
}

func (manager *mqttManager) GetUsername() string {
	return manager.Username
}

func (manager *mqttManager) GetVhost() string {
	return manager.Vhost
}

func (manager *mqttManager) IsDisabled() bool {
	return manager.disabled
}
