package gorabbit

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// amqpConnection holds information about the management of the native amqp.Connection.
type amqpConnection struct {
	// ctx is the parent context and acts as a safeguard.
	ctx context.Context

	// connection is the native amqp.Connection.
	connection *amqp.Connection

	// uri represents the connection string to the RabbitMQ server.
	uri string

	// keepAlive is the flag that will define whether active guards and re-connections are enabled or not.
	keepAlive bool

	// retryDelay defines the delay to wait before re-connecting if we lose connection and the keepAlive flag is set to true.
	retryDelay time.Duration

	// closed is an inner property that switches to true if the connection was explicitly closed.
	closed bool

	// channels holds a list of active amqpChannel
	channels amqpChannels

	// maxRetry defines the number of retries when publishing a message.
	maxRetry uint

	// publishingCacheSize defines the maximum length of cached failed publishing.
	publishingCacheSize uint64

	// publishingCacheTTL defines the time to live for a cached failed publishing.
	publishingCacheTTL time.Duration

	// logger logs events.
	logger Logger

	// connectionType defines the ConnectionType.
	connectionType ConnectionType
}

// newConsumerConnection initializes a new consumer amqpConnection with given arguments.
//   - ctx is the parent context.
//   - uri is the connection string.
//   - keepAlive will keep the connection alive if true.
//   - retryDelay defines the delay between each re-connection, if the keepAlive flag is set to true.
//   - logger is the parent logger.
func newConsumerConnection(ctx context.Context, uri string, keepAlive bool, retryDelay time.Duration, logger Logger) *amqpConnection {
	return newConnection(ctx, uri, keepAlive, retryDelay, logger, Consumer)
}

// newPublishingConnection initializes a new publisher amqpConnection with given arguments.
//   - ctx is the parent context.
//   - uri is the connection string.
//   - keepAlive will keep the connection alive if true.
//   - retryDelay defines the delay between each re-connection, if the keepAlive flag is set to true.
//   - maxRetry defines the publishing max retry header.
//   - publishingCacheSize defines the maximum length of failed publishing cache.
//   - publishingCacheTTL defines the time to live for failed publishing in cache.
//   - logger is the parent logger.
func newPublishingConnection(ctx context.Context, uri string, keepAlive bool, retryDelay time.Duration, maxRetry uint, publishingCacheSize uint64, publishingCacheTTL time.Duration, logger Logger) *amqpConnection {
	conn := newConnection(ctx, uri, keepAlive, retryDelay, logger, Publisher)

	conn.maxRetry = maxRetry
	conn.publishingCacheSize = publishingCacheSize
	conn.publishingCacheTTL = publishingCacheTTL

	return conn
}

// newConnection initializes a new amqpConnection with given arguments.
//   - ctx is the parent context.
//   - uri is the connection string.
//   - keepAlive will keep the connection alive if true.
//   - retryDelay defines the delay between each re-connection, if the keepAlive flag is set to true.
//   - logger is the parent logger.
func newConnection(ctx context.Context, uri string, keepAlive bool, retryDelay time.Duration, logger Logger, connectionType ConnectionType) *amqpConnection {
	conn := &amqpConnection{
		ctx:            ctx,
		uri:            uri,
		keepAlive:      keepAlive,
		retryDelay:     retryDelay,
		channels:       make(amqpChannels, 0),
		logger:         logger,
		connectionType: connectionType,
	}

	// We open an initial connection.
	err := conn.open()

	// If the connection failed and the keepAlive flag is set to true, we want to re-connect until success.
	if err != nil && keepAlive {
		conn.logger.Printf("Could not instantiate connection: %s. Trying again.", err.Error())

		go conn.reconnect()
	}

	return conn
}

// open opens a new amqp.Connection with the help of a defined uri.
func (a *amqpConnection) open() error {
	// If the uri is empty, we return an error.
	if a.uri == "" {
		return errEmptyURI
	}

	// We request a connection from the RabbitMQ server.
	conn, err := amqp.Dial(a.uri)
	if err != nil {
		return err
	}

	a.connection = conn

	a.channels.updateParentConnection(a.connection)

	// If the keepAlive flag is set to true but no guard is active, we activate a new guard.
	if a.keepAlive {
		go a.guard()
	}

	return nil
}

// reconnect will indefinitely call the open method until a connection is successfully established or the context is canceled.
func (a *amqpConnection) reconnect() {
	for {
		select {
		case <-a.ctx.Done():
			// If the context was canceled, we break out of the method.
			return
		default:
			// Wait for the retryDelay.
			time.Sleep(a.retryDelay)

			// If there is no connection or the current connection is closed, we open a new connection.
			if !a.ready() {
				err := a.open()
				// If the operation succeeds, we break the loop.
				if err == nil {
					return
				}

				a.logger.Printf("Could not reconnect: %s. Trying again.", err.Error())
			} else {
				// If the connection exists and is active, we break out.
				return
			}
		}
	}
}

// guard is a connection safeguard that listens to connection close events and re-launches the connection.
func (a *amqpConnection) guard() {
	for {
		select {
		case <-a.ctx.Done():
			// If the context was canceled, we break out of the method.
			return
		case err, ok := <-a.connection.NotifyClose(make(chan *amqp.Error)):
			if !ok {
				return
			}

			if err != nil {
				a.logger.Printf("Connection closed: %s.", err.Error())
			}

			// If the connection was explicitly closed, we do not want to re-connect.
			if a.closed {
				return
			}

			go a.reconnect()

			return
		}
	}
}

// close the connection only if it is ready.
func (a *amqpConnection) close() error {
	if a.ready() {
		for _, channel := range a.channels {
			err := channel.close()
			if err != nil {
				return err
			}
		}

		err := a.connection.Close()
		if err != nil {
			return err
		}

		a.closed = true

		return nil
	}

	return errConnectionClosed
}

// ready returns true of the connection exists and is not closed.
func (a *amqpConnection) ready() bool {
	return a.connection != nil && !a.connection.IsClosed()
}

// healthy returns true of the connection exists, is not closed and all child channels are healthy.
func (a *amqpConnection) healthy() bool {
	// If the connection is not ready, return false.
	if !a.ready() {
		return false
	}

	// Verify that all connection channels are ready too.
	for _, channel := range a.channels {
		if !channel.healthy() {
			return false
		}
	}

	return true
}

// registerConsumer opens a new consumerChannel and registers the MessageConsumer.
func (a *amqpConnection) registerConsumer(consumer MessageConsumer) error {
	for _, channel := range a.channels {
		if channel.consumer != nil && channel.consumer.Queue == consumer.Queue {
			return errConsumerAlreadyExists
		}
	}

	channel := newConsumerChannel(a.ctx, a.connection, a.keepAlive, a.retryDelay, &consumer, a.logger)

	a.channels = append(a.channels, channel)

	return nil
}

func (a *amqpConnection) publish(exchange, routingKey string, payload []byte, options *publishingOptions) error {
	publishingChannel := a.channels.publishingChannel()
	if publishingChannel == nil {
		publishingChannel = newPublishingChannel(a.ctx, a.connection, a.keepAlive, a.retryDelay, a.maxRetry, a.publishingCacheSize, a.publishingCacheTTL, a.logger)

		a.channels = append(a.channels, publishingChannel)
	}

	return publishingChannel.publish(exchange, routingKey, payload, options)
}
