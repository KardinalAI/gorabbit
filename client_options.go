package gorabbit

import (
	"time"

	"github.com/Netflix/go-env"
)

// ClientOptions holds all necessary properties to launch a successful connection with an MQTTClient.
type ClientOptions struct {
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

	// UseTLS defines whether we use amqp or amqps protocol.
	UseTLS bool

	// ConnectionName is the client connection name passed on to the RabbitMQ server.
	ConnectionName string

	// KeepAlive will determine whether the re-connection and retry mechanisms should be triggered.
	KeepAlive bool

	// RetryDelay will define the delay for the re-connection and retry mechanism.
	RetryDelay time.Duration

	// MaxRetry will define the number of retries when an amqpMessage could not be processed.
	MaxRetry uint

	// PublishingCacheTTL defines the time to live for each publishing cache item.
	PublishingCacheTTL time.Duration

	// PublishingCacheSize defines the max length of the publishing cache.
	PublishingCacheSize uint64

	// Mode will specify whether logs are enabled or not.
	Mode string

	// Marshaller defines the content type used for messages and how they're marshalled (default: JSON).
	Marshaller Marshaller
}

// DefaultClientOptions will return a ClientOptions with default values.
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		Host:                defaultHost,
		Port:                defaultPort,
		Username:            defaultUsername,
		Password:            defaultPassword,
		Vhost:               defaultVhost,
		UseTLS:              defaultUseTLS,
		KeepAlive:           defaultKeepAlive,
		RetryDelay:          defaultRetryDelay,
		MaxRetry:            defaultMaxRetry,
		PublishingCacheTTL:  defaultPublishingCacheTTL,
		PublishingCacheSize: defaultPublishingCacheSize,
		Mode:                defaultMode,
		Marshaller:          defaultMarshaller,
	}
}

// NewClientOptions is the exported builder for a ClientOptions and will offer setter methods for an easy construction.
// Any non-assigned field will be set to default through DefaultClientOptions.
func NewClientOptions() *ClientOptions {
	return DefaultClientOptions()
}

// NewClientOptionsFromEnv will generate a ClientOptions from environment variables. Empty values will be taken as default
// through the DefaultClientOptions.
func NewClientOptionsFromEnv() *ClientOptions {
	defaultOpts := DefaultClientOptions()

	fromEnv := new(RabbitMQEnvs)

	_, err := env.UnmarshalFromEnviron(fromEnv)
	if err != nil {
		return defaultOpts
	}

	if fromEnv.Host != "" {
		defaultOpts.Host = fromEnv.Host
	}

	if fromEnv.Port > 0 {
		defaultOpts.Port = fromEnv.Port
	}

	if fromEnv.Username != "" {
		defaultOpts.Username = fromEnv.Username
	}

	if fromEnv.Password != "" {
		defaultOpts.Password = fromEnv.Password
	}

	if fromEnv.Vhost != "" {
		defaultOpts.Vhost = fromEnv.Vhost
	}

	defaultOpts.UseTLS = fromEnv.UseTLS
	defaultOpts.ConnectionName = fromEnv.ConnectionName

	return defaultOpts
}

// SetHost will assign the Host.
func (c *ClientOptions) SetHost(host string) *ClientOptions {
	c.Host = host

	return c
}

// SetPort will assign the Port.
func (c *ClientOptions) SetPort(port uint) *ClientOptions {
	c.Port = port

	return c
}

// SetCredentials will assign the Username and Password.
func (c *ClientOptions) SetCredentials(username, password string) *ClientOptions {
	c.Username = username
	c.Password = password

	return c
}

// SetVhost will assign the Vhost.
func (c *ClientOptions) SetVhost(vhost string) *ClientOptions {
	c.Vhost = vhost

	return c
}

// SetUseTLS will assign the UseTLS status.
func (c *ClientOptions) SetUseTLS(use bool) *ClientOptions {
	c.UseTLS = use

	return c
}

// SetConnectionName will assign the ConnectionName.
func (c *ClientOptions) SetConnectionName(connectionName string) *ClientOptions {
	c.ConnectionName = connectionName

	return c
}

// SetKeepAlive will assign the KeepAlive status.
func (c *ClientOptions) SetKeepAlive(keepAlive bool) *ClientOptions {
	c.KeepAlive = keepAlive

	return c
}

// SetRetryDelay will assign the retry delay.
func (c *ClientOptions) SetRetryDelay(delay time.Duration) *ClientOptions {
	c.RetryDelay = delay

	return c
}

// SetMaxRetry will assign the max retry count.
func (c *ClientOptions) SetMaxRetry(retry uint) *ClientOptions {
	c.MaxRetry = retry

	return c
}

// SetPublishingCacheTTL will assign the publishing cache item TTL.
func (c *ClientOptions) SetPublishingCacheTTL(ttl time.Duration) *ClientOptions {
	c.PublishingCacheTTL = ttl

	return c
}

// SetPublishingCacheSize will assign the publishing cache max length.
func (c *ClientOptions) SetPublishingCacheSize(size uint64) *ClientOptions {
	c.PublishingCacheSize = size

	return c
}

// SetMode will assign the Mode if valid.
func (c *ClientOptions) SetMode(mode string) *ClientOptions {
	if isValidMode(mode) {
		c.Mode = mode
	}

	return c
}

// SetMarshaller will assign the Marshaller.
func (c *ClientOptions) SetMarshaller(marshaller Marshaller) *ClientOptions {
	c.Marshaller = marshaller

	return c
}
