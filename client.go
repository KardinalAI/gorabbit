package gorabbit

import (
	"context"
	"fmt"
	"os"
)

// MQTTClient is a simple MQTT interface that offers basic client operations such as:
//   - Publishing
//   - Consuming
//   - Disconnecting
//   - Ready and health checks
type MQTTClient interface {
	// Disconnect launches the disconnection process.
	// This operation disables to client permanently.
	Disconnect() error

	// Publish will send the desired payload through the selected channel.
	//	- exchange is the name of the exchange targeted for event publishing.
	//	- routingKey is the route that the exchange will use to forward the message.
	//	- payload is the object you want to send as a byte array.
	// Returns an error if the connection to the RabbitMQ server is down.
	Publish(exchange, routingKey string, payload interface{}) error

	// PublishWithOptions will send the desired payload through the selected channel.
	//	- exchange is the name of the exchange targeted for event publishing.
	//	- routingKey is the route that the exchange will use to forward the message.
	//	- payload is the object you want to send as a byte array.
	// Optionally you can add publishingOptions for extra customization.
	// Returns an error if the connection to the RabbitMQ server is down.
	PublishWithOptions(exchange, routingKey string, payload interface{}, options *PublishingOptions) error

	// RegisterConsumer will register a MessageConsumer for internal queue subscription and message processing.
	// The MessageConsumer will hold a list of MQTTMessageHandlers to internalize message processing.
	// Based on the return of error of each handler, the process of acknowledgment, rejection and retry of messages is
	// fully handled internally.
	// Furthermore, connection lost and channel errors are also internally handled by the connectionManager that will keep consumers
	// alive if and when necessary.
	RegisterConsumer(consumer MessageConsumer) error

	// IsReady returns true if the client is fully operational and connected to the RabbitMQ.
	IsReady() bool

	// IsHealthy returns true if the client is ready (IsReady) and all channels are operating successfully.
	IsHealthy() bool

	// GetHost returns the host used to initialize the client.
	GetHost() string

	// GetPort returns the port used to initialize the client.
	GetPort() uint

	// GetUsername returns the username used to initialize the client.
	GetUsername() string

	// GetVhost returns the vhost used to initialize the client.
	GetVhost() string

	// IsDisabled returns whether the client is disabled or not.
	IsDisabled() bool
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

	// Vhost is used for CloudAMQP connections to set the specific vhost.
	Vhost string

	// logger defines the logger used, depending on the mode set.
	logger logger

	// disabled completely disables the client if true.
	disabled bool

	// connectionManager manages the connection and channel logic and high-level logic
	// such as keep alive mechanism and health check.
	connectionManager *connectionManager

	// ctx holds the global context used for the client.
	ctx context.Context

	// cancel is the cancelFunc for the ctx.
	cancel context.CancelFunc
}

// NewClient will instantiate a new MQTTClient.
// If options is set to nil, the DefaultClientOptions will be used.
func NewClient(options *ClientOptions) MQTTClient {
	// If no options is passed, we use the DefaultClientOptions.
	if options == nil {
		options = DefaultClientOptions()
	}

	return newClientFromOptions(options)
}

// NewClientFromEnv will instantiate a new MQTTClient from environment variables.
func NewClientFromEnv() MQTTClient {
	options := NewClientOptionsFromEnv()

	return newClientFromOptions(options)
}

func newClientFromOptions(options *ClientOptions) MQTTClient {
	client := &mqttClient{
		Host:     options.Host,
		Port:     options.Port,
		Username: options.Username,
		Password: options.Password,
		Vhost:    options.Vhost,
		logger:   &noLogger{},
	}

	// We check if the disabled flag is present, which will completely disable the MQTTClient.
	if disabledOverride := os.Getenv("GORABBIT_DISABLED"); disabledOverride != "" {
		switch disabledOverride {
		case "1", "true":
			client.disabled = true

			return client
		}
	}

	// We check if the mode was overwritten with the environment variable "GORABBIT_MODE".
	if modeOverride := os.Getenv("GORABBIT_MODE"); isValidMode(modeOverride) {
		// We override the mode only if it is valid
		options.Mode = modeOverride
	}

	if options.Mode == Debug {
		// If the mode is Debug, we want to actually log important events.
		client.logger = newStdLogger()
	}

	client.ctx, client.cancel = context.WithCancel(context.Background())

	protocol := defaultProtocol

	if options.UseTLS {
		protocol = securedProtocol
	}

	dialURL := fmt.Sprintf("%s://%s:%s@%s:%d/%s", protocol, client.Username, client.Password, client.Host, client.Port, client.Vhost)

	client.connectionManager = newConnectionManager(
		client.ctx,
		dialURL,
		options.KeepAlive,
		options.RetryDelay,
		options.MaxRetry,
		options.PublishingCacheSize,
		options.PublishingCacheTTL,
		client.logger,
	)

	return client
}

func (client *mqttClient) Publish(exchange string, routingKey string, payload interface{}) error {
	return client.PublishWithOptions(exchange, routingKey, payload, nil)
}

func (client *mqttClient) PublishWithOptions(exchange string, routingKey string, payload interface{}, options *PublishingOptions) error {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return nil
	}

	return client.connectionManager.publish(exchange, routingKey, payload, options)
}

func (client *mqttClient) RegisterConsumer(consumer MessageConsumer) error {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return nil
	}

	return client.connectionManager.registerConsumer(consumer)
}

func (client *mqttClient) Disconnect() error {
	// client is disabled, so we do nothing and return no error.
	if client.disabled {
		return nil
	}

	err := client.connectionManager.close()

	if err != nil {
		return err
	}

	// cancel the context to stop all reconnection goroutines.
	client.cancel()

	// disable the client to avoid trying to launch new operations.
	client.disabled = true

	return nil
}

func (client *mqttClient) IsReady() bool {
	// client is disabled, so we do nothing and return true.
	if client.disabled {
		return true
	}

	return client.connectionManager.isReady()
}

func (client *mqttClient) IsHealthy() bool {
	// client is disabled, so we do nothing and return true.
	if client.disabled {
		return true
	}

	return client.connectionManager.isHealthy()
}

func (client *mqttClient) GetHost() string {
	return client.Host
}

func (client *mqttClient) GetPort() uint {
	return client.Port
}

func (client *mqttClient) GetUsername() string {
	return client.Username
}

func (client *mqttClient) GetVhost() string {
	return client.Vhost
}

func (client *mqttClient) IsDisabled() bool {
	return client.disabled
}
