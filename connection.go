package gorabbit

import (
	"context"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionManager struct {
	// uri represents the connection string to the RabbitMQ server.
	uri string

	// ctx will be use as the main context for all functionalities.
	ctx context.Context

	consumptionCtx context.Context

	consumptionCancel context.CancelFunc

	// connection manages channels to handle operations such as message receptions and publishing.
	connection *amqp.Connection

	// channel is responsible for sending and receiving amqpMessage as well as other low level methods.
	channel *amqp.Channel

	// keepAlive will determine whether the re-connection and retry mechanisms should be triggered.
	keepAlive bool

	// retryDelay will define the delay for the re-connection and retry mechanism.
	retryDelay time.Duration

	// maxRetry will define the number of retries when an amqpMessage could not be processed.
	maxRetry uint

	// subscriptionsHealth manages the status of all active subscriptionsHealth.
	subscriptionsHealth subscriptionsHealth

	// consumers manages the registered consumptions.
	consumers map[string]MessageConsumer

	// publishingCache manages the caching of unpublished messages due to a connection error.
	publishingCache *ttlMap[string, mqttPublishing]

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
	logger Logger,
) *connectionManager {
	c := &connectionManager{
		uri:                 uri,
		ctx:                 ctx,
		keepAlive:           keepAlive,
		retryDelay:          retryDelay,
		maxRetry:            maxRetry,
		subscriptionsHealth: make(subscriptionsHealth),
		consumers:           make(map[string]MessageConsumer),
		publishingCache:     newTTLMap[string, mqttPublishing](publishingCacheMaxLength, publishingCacheTTL),
		logger:              logger,
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
			select {
			case <-c.ctx.Done():
				c.logger.Printf("Cannot try to connect because the context is done")
				return
			default:
				err := c.newConnection()
				if err != nil {
					c.logger.Printf("Could not create new connection: %s", err.Error())
					c.registerStatusChange(ConnFailed)
					time.Sleep(retryDelay)
				} else {
					return
				}
			}
		}
	}()

	return c
}

// newConnection instantiates the connection to RabbitMQ and requests a channel. Retry and KeepAlive mechanisms are also triggered.
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

// newChannel instantiates a new channel. Retry and KeepAlive mechanisms are also triggered.
func (c *connectionManager) newChannel() error {
	// We request a channel from the previously opened connection.
	ch, err := c.connection.Channel()

	if err != nil {
		return err
	}

	c.registerStatusChange(ChanUp)

	c.channel = ch

	// We make sure to empty publishing cache once the channel is up.
	go c.emptyPublishingCache()

	// We know for sure that the consumptionCtx was non-existent or canceled on ChanDown event, so we instantiate a new one.
	c.consumptionCtx, c.consumptionCancel = context.WithCancel(c.ctx)

	for _, consumer := range c.consumers {
		// We launch consumptions once channel is up.
		go c.beginConsumption(consumer)
	}

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

			// We want to cancel the consumptionCtx to stop consumers from consuming a queue without a channel opened.
			c.consumptionCancel()

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

						// We know for sure that the consumptionCtx was canceled on ChanDown event, so we instantiate a new one.
						c.consumptionCtx, c.consumptionCancel = context.WithCancel(c.ctx)

						for _, consumer := range c.consumers {
							go c.beginConsumption(consumer)
						}

						break Loop
					}
				}
			}
		}
	}
}

// registerStatusChange will trigger a simple log on ConnectionStatus change.
func (c *connectionManager) registerStatusChange(status ConnectionStatus) {
	switch status {
	case ConnFailed:
		c.logger.Printf("Connection to RabbitMQ server failed")
	case ConnUp:
		c.logger.Printf("Connection to RabbitMQ succeeded")
	case ChanUp:
		c.logger.Printf("Client channel is up")
	case ChanDown:
		c.logger.Printf("Client channel is down")
	case ConnDown:
		c.logger.Printf("Connection to RabbitMQ server lost")
	case ConnClosed:
		c.logger.Printf("Client disconnected from the RabbitMQ server")
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

// isHealthy return true if all registered subscriptionsHealth succeeded.
// Returns false if one or more subscription failed.
func (c *connectionManager) isHealthy() bool {
	return c.subscriptionsHealth.IsHealthy()
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

// registerConsumer registers a new MessageConsumer and caches it inside the consumers cache (map).
func (c *connectionManager) registerConsumer(consumer MessageConsumer) {
	// If the consumer was not previously registered, we register it.
	if _, ok := c.consumers[consumer.HashCode()]; !ok {
		// Insert the consumer in cache (map).
		c.consumers[consumer.HashCode()] = consumer

		// If the connectionManager is operational, we can already start consuming.
		if c.isOperational() {
			// We consume asynchronously, otherwise this operation would be blocking.
			go c.beginConsumption(consumer)
		}
	}
}

// beginConsumption handles the consumption mechanism for a given MessageConsumer.
func (c *connectionManager) beginConsumption(consumer MessageConsumer) {
	messages, err := c.Consume(consumer.Queue, consumer.Consumer, consumer.AutoAck, false, false, false, nil)
	if err != nil {
		// Check if the error is of type amqp.Error.
		var amqpErr *amqp.Error

		ok := errors.As(err, &amqpErr)

		// If the error is an amqp.Error and is of type not found, we want to remove the consumer from cache otherwise,
		// we risk an infinite loop of de-connection and re-connection to consume a non-existent queue.
		if ok && amqpErr != nil && amqpErr.Code == amqp.NotFound {
			c.logger.Printf("Queue '%s' does not exist", consumer.Queue)

			delete(c.consumers, consumer.HashCode())

			return
		}

		c.logger.Printf("Could not subscribe to queue '%s': %s", consumer.Queue, err.Error())

		return
	}

	// If there is no error, we want to loop through every message received, indefinitely.
	for {
		select {
		case <-c.consumptionCtx.Done():
			// If the consumptionCtx was canceled (connection lost or channel closed), we stop consuming.
			c.logger.Printf("Finished consuming messages from queue '%s'", consumer.Queue)
			return
		case message := <-messages:
			// When a queue is deleted midway, a message with no delivery tag or ID is received.
			if message.DeliveryTag == 0 && message.MessageId == "" {
				c.logger.Printf("Queue '%s' was deleted", consumer.Queue)

				// We want to remove the consumer to avoid getting into an infinite loop of consumption retry.
				delete(c.consumers, consumer.HashCode())

				return
			}

			c.logger.Printf("Received amqp delivery with tag %d and id %s", message.DeliveryTag, message.MessageId)

			// We parse the message to extract information such as Microservice, Entity and Action.
			parsed, parseErr := ParseMessage(message)

			// If the message could not be parsed, we call the OnBadFormat hook if it exists.
			if parseErr != nil {
				c.logger.Printf("Could not parse AMQP message, calling bad format hook")

				if consumer.OnBadFormat != nil {
					consumer.OnBadFormat(message.RoutingKey, message.Body)
				}

				continue
			}

			// nolint: nestif // The complexity is necessary here.
			// If a handler is found for the message's routingKey, we execute it.
			if handler, found := consumer.Handlers[parsed.RoutingKey]; found {
				err = handler(parsed.Body)

				switch {
				// If there was an error processing the message, but the retry mechanism was disabled.
				case err != nil && c.maxRetry == 0:
					nackErr := parsed.Reject(false)
					if nackErr != nil {
						c.logger.Printf("Could not Nack message with ID %s: %s", parsed.MessageId, nackErr)
					}
				// If there was an error processing the message and the retry mechanism was enabled.
				case err != nil && c.maxRetry > 0:
					retryErr := c.retryMessage(parsed, c.maxRetry)

					// If the retry mechanism failed, we call the OnRetryError hook if it exists.
					if retryErr != nil {
						c.logger.Printf("Could not retry message with ID %s: %s", parsed.MessageId, retryErr)

						if consumer.OnRetryError != nil {
							consumer.OnRetryError(parsed.MessageId, retryErr)
						}
					}
				// If there was no error.
				default:
					ackErr := parsed.Ack(false)
					if ackErr != nil {
						c.logger.Printf("Could not Ack message with ID %s: %s", parsed.MessageId, ackErr)
					}
				}
			}
		}
	}
}

// retryMessage offers the possibility of retrying a message that could not be processed, a given number of times.
func (c *connectionManager) retryMessage(event *amqpMessage, maxRetry uint) error {
	// We first acknowledge the message to remove it from the queue.
	if err := event.Ack(false); err != nil {
		c.logger.Printf("Could not Ack message with ID %s", event.MessageId)

		return err
	}

	// We increment the redelivery count in the message's header.
	redeliveredCount := event.IncrementRedeliveryHeader()

	c.logger.Printf("Incremented redelivered count to %d", redeliveredCount)

	// If the redelivery count header is still below the maximum allowed.
	if redeliveredCount < int32(maxRetry) {
		c.logger.Printf("Redelivering message")

		return c.Publish(
			event.Exchange,
			event.RoutingKey,
			false,
			false,
			event.ToPublishing(),
			false,
		)
	}

	// Otherwise, we return a max retry reached error.
	return errMaxRetryReached
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

	c.subscriptionsHealth.AddSubscription(queue, err)

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
