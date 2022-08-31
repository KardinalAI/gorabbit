package gorabbit

import "time"

// Default values for the clientOptions.
const (
	defaultHost       = "127.0.0.1"
	defaultPort       = 5672
	defaultUsername   = "guest"
	defaultPassword   = "guest"
	defaultKeepAlive  = true
	defaultRetryDelay = 3 * time.Second
	defaultMaxRetry   = 5
	defaultMode       = Release
)

// clientOptions is an unexported type that holds all necessary properties to launch a successful connection with an MQTTClient.
type clientOptions struct {
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

	// maxRetry will define the number of retries when an AMQPMessage could not be processed.
	maxRetry uint

	// mode will specify whether logs are enabled or not.
	mode string
}

// DefaultClientOptions will return a clientOptions with default values.
func DefaultClientOptions() *clientOptions {
	return &clientOptions{
		host:       defaultHost,
		port:       defaultPort,
		username:   defaultUsername,
		password:   defaultPassword,
		keepAlive:  defaultKeepAlive,
		retryDelay: defaultRetryDelay,
		maxRetry:   defaultMaxRetry,
		mode:       defaultMode,
	}
}

// NewClientOptions is the exported builder for a clientOptions and will offer setter methods for an easy construction.
// Any non-assigned field will be set to default through DefaultClientOptions.
func NewClientOptions() *clientOptions {
	return DefaultClientOptions()
}

// SetHost will assign the host.
func (c *clientOptions) SetHost(host string) *clientOptions {
	c.host = host

	return c
}

// SetPort will assign the port.
func (c *clientOptions) SetPort(port uint) *clientOptions {
	c.port = port

	return c
}

// SetCredentials will assign the username and password.
func (c *clientOptions) SetCredentials(username, password string) *clientOptions {
	c.username = username
	c.password = password

	return c
}

// SetKeepAlive will assign the keepAlive status.
func (c *clientOptions) SetKeepAlive(keepAlive bool) *clientOptions {
	c.keepAlive = keepAlive

	return c
}

// SetRetryDelay will assign the retry delay.
func (c *clientOptions) SetRetryDelay(delay time.Duration) *clientOptions {
	c.retryDelay = delay

	return c
}

// SetMaxRetry will assign the max retry count.
func (c *clientOptions) SetMaxRetry(retry uint) *clientOptions {
	c.maxRetry = retry

	return c
}

// SetMode will assign the mode if valid.
func (c *clientOptions) SetMode(mode string) *clientOptions {
	if isValidMode(mode) {
		c.mode = mode
	}

	return c
}
