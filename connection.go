package gorabbit

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionManager struct {
	// uri represents the connection string to the RabbitMQ server.
	uri string

	// ctx will be use as the main context for all functionalities.
	ctx context.Context

	// connection manages channels to handle operations such as message receptions and publishing.
	connection *amqp.Connection

	// channel is responsible for sending and receiving AMQPMessage as well as other low level methods.
	channel *amqp.Channel

	// keepAlive will determine whether the re-connection and retry mechanisms should be triggered.
	keepAlive bool

	// retryDelay will define the delay for the re-connection and retry mechanism.
	retryDelay time.Duration

	// maxRetry will define the number of retries when an AMQPMessage could not be processed.
	maxRetry uint

	// subscriptions manages the status of all active subscriptions.
	subscriptions subscriptionsHealth

	// publishingCache manages the caching of unpublished messages due to a connection error.
	publishingCache *ttlMap[string, mqttPublishing]

	// statusListeners holds listeners for each ConnectionStatus change.
	statusListeners *ClientListeners

	// logger is passed from the client for debugging purposes.
	logger Logger
}

// newManager instantiates a new connectionManager with given arguments.
func newManager(
	ctx context.Context,
	uri string,
	keepAlive bool,
	retryDelay time.Duration,
	maxRetry uint,
	publishingCacheMaxLength uint64,
	publishingCacheTTL time.Duration,
	statusListeners *ClientListeners,
	logger Logger,
) *connectionManager {
	c := &connectionManager{
		uri:             uri,
		ctx:             ctx,
		keepAlive:       keepAlive,
		retryDelay:      retryDelay,
		maxRetry:        maxRetry,
		subscriptions:   make(subscriptionsHealth),
		publishingCache: newTTLMap[string, mqttPublishing](publishingCacheMaxLength, publishingCacheTTL),
		statusListeners: statusListeners,
		logger:          logger,
	}

	// If we don't want to keep trying to connect in case a first attempt fails, we instantiate a new connection and return an error if the operation failed.
	if !c.keepAlive {
		if err := c.newConnection(); err != nil {
			c.logger.Printf("could not create new connection: %s", err.Error())
			c.registerStatusChange(ConnFailed)
		}

		return c
	}

	// If we want to keep trying to connect after a failure, we asynchronously keep launching a connection request until it succeeds.
	go func() {
		for {
			err := c.newConnection()
			if err != nil {
				c.logger.Printf("could not create new connection: %s", err.Error())
				c.registerStatusChange(ConnFailed)
				time.Sleep(retryDelay)
			} else {
				break
			}
		}
	}()

	return c
}

func (c *connectionManager) newConnection() error {
	// If the connection string is empty we return an error.
	if c.uri == "" {
		return errEmptyURI
	}

	// Only if the connection is nil or closed, should we try to connect.
	if c.connection == nil || c.connection.IsClosed() {
		// amqp.Dial will return a connection and an error.
		conn, err := amqp.Dial(c.uri)

		if err != nil {
			return err
		}

		c.registerStatusChange(ConnUp)

		c.connection = conn

		// If the connection request was successful, and we want to keep it up in case of failure, we call an asynchronous keepAlive method.
		if c.keepAlive {
			go c.keepConnectionAlive()
		}
	}

	// Finally, we return a new channel once the connection is established.
	return c.newChannel()
}

func (c *connectionManager) newChannel() error {
	// We request a channel from the previously opened connection.
	ch, err := c.connection.Channel()

	if err != nil {
		return err
	}

	c.registerStatusChange(ChanUp)

	c.channel = ch

	go c.emptyPublishingCache()

	const prefetchCount = 10

	const prefetchSize = 0

	// Calling Qos will allow the client to process the given amount of messages without the need to be acknowledged. Basically the number of message the client can process at the same time.
	err = c.channel.Qos(prefetchCount, prefetchSize, false)

	if err != nil {
		c.logger.Printf("could not declare QOS with prefetch count of %d", prefetchCount)
		return err
	}

	// If the channel request was successful, and we want to keep it up in case of failure, we call an asynchronous keepAlive method.
	if c.keepAlive {
		go c.keepChannelAlive()
	}

	return nil
}

// registerStatusChange will trigger an action on ConnectionStatus change.
// Every ConnectionStatus has its own optional listener, that is triggered only if declared.
func (c *connectionManager) registerStatusChange(status ConnectionStatus) {
	if c.statusListeners == nil {
		return
	}

	switch status {
	case ConnFailed:
		if c.statusListeners.OnConnectionFailed != nil {
			c.statusListeners.OnConnectionFailed()
		}
	case ConnUp:
		if c.statusListeners.OnConnectionUp != nil {
			c.statusListeners.OnConnectionUp()
		}
	case ChanUp:
		if c.statusListeners.OnChannelUp != nil {
			c.statusListeners.OnChannelUp()
		}
	case ChanDown:
		if c.statusListeners.OnChannelDown != nil {
			c.statusListeners.OnChannelDown()
		}
	case ConnDown:
		if c.statusListeners.OnConnectionLost != nil {
			c.statusListeners.OnConnectionLost()
		}
	case ConnClosed:
		if c.statusListeners.OnConnectionClosed != nil {
			c.statusListeners.OnConnectionClosed()
		}
	}
}

// close offers the basic connection and channel close() mechanism but with extra higher level checks.
func (c *connectionManager) close() error {
	// We only close a channel if it exists and is not already closed.
	if c.channel != nil && !c.channel.IsClosed() {
		err := c.channel.Close()

		if err != nil {
			return err
		}

		if !c.keepAlive {
			c.registerStatusChange(ChanDown)
		}

		c.channel = nil
	}

	// We only close a connection after child channel was closed, the connection exists and is not already closed.
	if c.connection != nil && !c.connection.IsClosed() {
		err := c.connection.Close()

		if err != nil {
			return err
		}

		if !c.keepAlive {
			c.registerStatusChange(ConnDown)
		}
	}

	c.connection = nil

	c.registerStatusChange(ConnClosed)

	return nil
}

// isOperational returns true if the connection and channel are up and running.
// Returns false if either the connection or channel is closed or non-existent.
func (c *connectionManager) isOperational() bool {
	if c.connection == nil {
		return false
	}

	if c.connection.IsClosed() {
		return false
	}

	if c.channel == nil {
		return false
	}

	if c.channel.IsClosed() {
		return false
	}

	return true
}

// isHealthy return true if all registered subscriptions succeeded.
// Returns false if one or more subscription failed.
func (c *connectionManager) isHealthy() bool {
	return c.subscriptions.IsHealthy()
}

// keepConnectionAlive listens to a connection close notification and tries to re-connect after waiting for a given retryDelay.
// This mechanism runs indefinitely or until the client is closed.
func (c *connectionManager) keepConnectionAlive() {
	for {
		select {
		case <-c.ctx.Done():
			c.logger.Printf("Cannot keep connection alive because the context is done")
			return
		case err, ok := <-c.connection.NotifyClose(make(chan *amqp.Error)):
			if err != nil {
				c.logger.Printf("Connection closed: %s", err.Error())
			}

			c.registerStatusChange(ConnDown)

			if !ok {
				return
			}

		Loop:
			for {
				// Wait reconnectDelay before trying to reconnect.
				time.Sleep(c.retryDelay)

				select {
				case <-c.ctx.Done():
					c.logger.Printf("Cannot keep connection alive because the context is done")
					return
				default:
					var err error
					c.connection, err = amqp.Dial(c.uri)
					if err == nil {
						c.registerStatusChange(ConnUp)
						break Loop
					}
				}
			}
		}
	}
}

// keepChannelAlive listens to a channel close notification and tries to request a new channel after waiting for a given retryDelay.
// This mechanism runs indefinitely or until the client is closed.
func (c *connectionManager) keepChannelAlive() {
	for {
		select {
		case <-c.ctx.Done():
			c.logger.Printf("Cannot keep channel alive because the context is done")
			return
		case err, ok := <-c.channel.NotifyClose(make(chan *amqp.Error)):
			if err != nil {
				c.logger.Printf("Channel closed: %s", err.Error())
			}

			c.registerStatusChange(ChanDown)

			if !ok {
				return
			}

		Loop:
			for {
				// Wait reconnectDelay before trying to reconnect.
				time.Sleep(c.retryDelay)

				select {
				case <-c.ctx.Done():
					c.logger.Printf("Cannot keep channel alive because the context is done")
					return
				default:
					if c.connection == nil || c.connection.IsClosed() {
						c.logger.Printf("Cannot create new channel without a connection")
						continue
					}

					c.logger.Printf("Trying to create a new channel")

					var err error
					c.channel, err = c.connection.Channel()
					if err == nil {
						c.registerStatusChange(ChanUp)
						go c.emptyPublishingCache()

						break Loop
					}
				}
			}
		}
	}
}

// emptyPublishingCache re-sends all cached mqttPublishing that failed due to a disconnection error, as soon as the connection is up again.
// This method is only a safeguard and contrary to a connection and channel, doesn't run indefinitely.
func (c *connectionManager) emptyPublishingCache() {
	// If the cache is empty, there is nothing to send.
	if c.publishingCache.Len() == 0 {
		return
	}

	c.logger.Printf("Emptying publishing cache...")

	// For each cached mqttPublishing, re-send the message only once then remove it from cache no matter whether it succeeds or fails.
	c.publishingCache.ForEach(func(k string, msg mqttPublishing) {
		c.logger.Printf("Re-sending message ID %s", k)
		_ = c.Publish(msg.Exchange, msg.RoutingKey, msg.Mandatory, msg.Immediate, msg.Msg, true)

		c.publishingCache.Delete(k)
	})

	c.logger.Printf("Publishing cache emptied")
}

// Publish does the same as the native amqp publish method, but with some higher level functionalities.
func (c *connectionManager) Publish(exchange, routingKey string, mandatory, immediate bool, msg amqp.Publishing, fromCache bool) error {
	if !c.isOperational() {
		// If the publishing is not coming from publishingCache, we want to insert it in the publishing cache.
		if !fromCache {
			mqttMsg := mqttPublishing{
				Exchange:   exchange,
				RoutingKey: routingKey,
				Mandatory:  mandatory,
				Immediate:  immediate,
				Msg:        msg,
			}

			c.logger.Printf("Caching unsuccessful publishing ID %s", msg.MessageId)

			c.publishingCache.Put(mqttMsg.HashCode(), mqttMsg)
		}

		return errConnectionOrChannelClosed
	}

	return c.channel.PublishWithContext(
		c.ctx,
		exchange,
		routingKey,
		mandatory,
		immediate,
		msg,
	)
}

// Consume does the same as the native amqp consume method, but checks for the connectionManager operational status first.
func (c *connectionManager) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	if !c.isOperational() {
		return nil, errConnectionOrChannelClosed
	}

	messages, err := c.channel.Consume(
		queue,
		consumer,
		autoAck,
		exclusive,
		noLocal,
		noWait,
		args,
	)

	c.subscriptions.AddSubscription(queue, err)

	return messages, err
}

// ExchangeDeclare does the same as the native amqp exchangeDeclare method, but checks for the connectionManager operational status first.
func (c *connectionManager) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	if !c.isOperational() {
		return errConnectionOrChannelClosed
	}

	return c.channel.ExchangeDeclare(
		name,
		kind,
		durable,
		autoDelete,
		internal,
		noWait,
		args,
	)
}

// ExchangeDelete does the same as the native amqp exchangeDelete method, but checks for the connectionManager operational status first.
func (c *connectionManager) ExchangeDelete(name string, ifUnused, noWait bool) error {
	if !c.isOperational() {
		return errConnectionOrChannelClosed
	}

	return c.channel.ExchangeDelete(
		name,
		ifUnused,
		noWait,
	)
}

// QueueDeclare does the same as the native amqp queueDeclare method, but checks for the connectionManager operational status first.
func (c *connectionManager) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if !c.isOperational() {
		return amqp.Queue{}, errConnectionOrChannelClosed
	}

	return c.channel.QueueDeclare(
		name,
		durable,
		autoDelete,
		exclusive,
		noWait,
		args,
	)
}

// QueueDeclarePassive does the same as the native amqp queueDeclarePassive method, but checks for the connectionManager operational status first.
func (c *connectionManager) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if !c.isOperational() {
		return amqp.Queue{}, errConnectionOrChannelClosed
	}

	return c.channel.QueueDeclarePassive(
		name,
		durable,
		autoDelete,
		exclusive,
		noWait,
		args,
	)
}

// QueueBind does the same as the native amqp queueBind method, but checks for the connectionManager operational status first.
func (c *connectionManager) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	if !c.isOperational() {
		return errConnectionOrChannelClosed
	}

	return c.channel.QueueBind(
		name,
		key,
		exchange,
		noWait,
		args,
	)
}

// QueuePurge does the same as the native amqp queuePurge method, but checks for the connectionManager operational status first.
func (c *connectionManager) QueuePurge(name string, noWait bool) (int, error) {
	if !c.isOperational() {
		return 0, errConnectionOrChannelClosed
	}

	return c.channel.QueuePurge(
		name,
		noWait,
	)
}

// QueueDelete does the same as the native amqp queueDelete method, but checks for the connectionManager operational status first.
func (c *connectionManager) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {
	if !c.isOperational() {
		return 0, errConnectionOrChannelClosed
	}

	return c.channel.QueueDelete(
		name,
		ifUnused,
		ifEmpty,
		noWait,
	)
}

// Get does the same as the native amqp get method, but checks for the connectionManager operational status first.
func (c *connectionManager) Get(queue string, autoAck bool) (amqp.Delivery, bool, error) {
	if !c.isOperational() {
		return amqp.Delivery{}, false, errConnectionOrChannelClosed
	}

	return c.channel.Get(queue, autoAck)
}
