package gorabbit

import "time"

// ClientOptions holds all necessary properties to launch a successful connection with an MQTTClient.
type ClientOptions struct {
	// host is the RabbitMQ server host name.
	host string

	// port is the RabbitMQ server port number.
	port uint

	// username is the RabbitMQ server allowed username.
	username string

	// password is the RabbitMQ server allowed password.
	password string

	// keepAlive will determine whether the re-connection and retry mechanisms should be triggered.
	keepAlive bool

	// retryDelay will define the delay for the re-connection and retry mechanism.
	retryDelay time.Duration

	// maxRetry will define the number of retries when an amqpMessage could not be processed.
	maxRetry uint

	// publishingCacheTTL defines the time to live for each publishing cache item.
	publishingCacheTTL time.Duration

	// publishingCacheSize defines the max length of the publishing cache.
	publishingCacheSize uint64

	// mode will specify whether logs are enabled or not.
	mode string
}

// DefaultClientOptions will return a ClientOptions with default values.
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		host:                defaultHost,
		port:                defaultPort,
		username:            defaultUsername,
		password:            defaultPassword,
		keepAlive:           defaultKeepAlive,
		retryDelay:          defaultRetryDelay,
		maxRetry:            defaultMaxRetry,
		publishingCacheTTL:  defaultPublishingCacheTTL,
		publishingCacheSize: defaultPublishingCacheSize,
		mode:                defaultMode,
	}
}

// NewClientOptions is the exported builder for a ClientOptions and will offer setter methods for an easy construction.
// Any non-assigned field will be set to default through DefaultClientOptions.
func NewClientOptions() *ClientOptions {
	return DefaultClientOptions()
}

// SetHost will assign the host.
func (c *ClientOptions) SetHost(host string) *ClientOptions {
	c.host = host

	return c
}

// SetPort will assign the port.
func (c *ClientOptions) SetPort(port uint) *ClientOptions {
	c.port = port

	return c
}

// SetCredentials will assign the username and password.
func (c *ClientOptions) SetCredentials(username, password string) *ClientOptions {
	c.username = username
	c.password = password

	return c
}

// SetKeepAlive will assign the keepAlive status.
func (c *ClientOptions) SetKeepAlive(keepAlive bool) *ClientOptions {
	c.keepAlive = keepAlive

	return c
}

// SetRetryDelay will assign the retry delay.
func (c *ClientOptions) SetRetryDelay(delay time.Duration) *ClientOptions {
	c.retryDelay = delay

	return c
}

// SetMaxRetry will assign the max retry count.
func (c *ClientOptions) SetMaxRetry(retry uint) *ClientOptions {
	c.maxRetry = retry

	return c
}

// SetPublishingCacheTTL will assign the publishing cache item TTL.
func (c *ClientOptions) SetPublishingCacheTTL(ttl time.Duration) *ClientOptions {
	c.publishingCacheTTL = ttl

	return c
}

// SetPublishingCacheSize will assign the publishing cache max length.
func (c *ClientOptions) SetPublishingCacheSize(size uint64) *ClientOptions {
	c.publishingCacheSize = size

	return c
}

// SetMode will assign the mode if valid.
func (c *ClientOptions) SetMode(mode string) *ClientOptions {
	if isValidMode(mode) {
		c.mode = mode
	}

	return c
}
