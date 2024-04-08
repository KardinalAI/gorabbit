package gorabbit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	amqp "github.com/rabbitmq/amqp091-go"
)

// amqpChannels is a simple wrapper of an amqpChannel slice.
type amqpChannels []*amqpChannel

// publishingChannel loops through all channels and returns the first available publisher channel if it exists.
func (a amqpChannels) publishingChannel() *amqpChannel {
	for _, channel := range a {
		if channel != nil && channel.connectionType == connectionTypePublisher {
			return channel
		}
	}

	return nil
}

// updateParentConnection updates every channel's parent connection.
func (a amqpChannels) updateParentConnection(conn *amqp.Connection) {
	for _, channel := range a {
		channel.connection = conn
	}
}

// amqpChannel holds information about the management of the native amqp.Channel.
type amqpChannel struct {
	// ctx is the parent context and acts as a safeguard.
	ctx context.Context

	// connection is the native amqp.Connection.
	connection *amqp.Connection

	// channel is the native amqp.Channel.
	channel *amqp.Channel

	// keepAlive is the flag that will define whether active guards and re-connections are enabled or not.
	keepAlive bool

	// retryDelay defines the delay to wait before re-connecting if the channel was closed and the keepAlive flag is set to true.
	retryDelay time.Duration

	// consumer is the MessageConsumer that holds all necessary information for the consumption of messages.
	consumer *MessageConsumer

	// consumptionCtx holds the consumption context.
	consumptionCtx context.Context

	// consumptionCancel is the cancel function of the consumptionCtx.
	consumptionCancel context.CancelFunc

	// consumptionHealth manages the status of all active consumptions.
	consumptionHealth consumptionHealth

	// publishingCache manages the caching of unpublished messages due to a connection error.
	publishingCache *ttlMap[string, mqttPublishing]

	// maxRetry defines the retry header for each message.
	maxRetry uint

	// closed is an inner property that switches to true if the channel was explicitly closed.
	closed bool

	// logger logs events.
	logger logger

	// releaseLogger forces logs not matter the mode. It is used to log important things.
	releaseLogger logger

	// connectionType defines the connectionType.
	connectionType connectionType
}

// newConsumerChannel instantiates a new consumerChannel and amqpChannel for method inheritance.
//   - ctx is the parent context.
//   - connection is the parent amqp.Connection.
//   - keepAlive will keep the channel alive if true.
//   - retryDelay defines the delay between each retry, if the keepAlive flag is set to true.
//   - consumer is the MessageConsumer that will hold consumption information.
//   - maxRetry is the retry header for each message.
//   - logger is the parent logger.
func newConsumerChannel(
	ctx context.Context,
	connection *amqp.Connection,
	keepAlive bool,
	retryDelay time.Duration,
	consumer *MessageConsumer,
	logger logger,
) *amqpChannel {
	channel := &amqpChannel{
		ctx:        ctx,
		connection: connection,
		keepAlive:  keepAlive,
		retryDelay: retryDelay,
		logger: inheritLogger(logger, map[string]interface{}{
			"context":  "channel",
			"type":     connectionTypeConsumer,
			"consumer": consumer.Name,
			"queue":    consumer.Queue,
		}),
		releaseLogger: &stdLogger{
			logger:     newLogrus(),
			identifier: libraryName,
			logFields: map[string]interface{}{
				"context":  "channel",
				"type":     connectionTypeConsumer,
				"consumer": consumer.Name,
				"queue":    consumer.Queue,
			},
		},
		connectionType:    connectionTypeConsumer,
		consumptionHealth: make(consumptionHealth),
		consumer:          consumer,
	}

	// We open an initial channel.
	err := channel.open()

	// If the channel failed to open and the keepAlive flag is set to true, we want to retry until success.
	if err != nil && keepAlive {
		go channel.retry()
	}

	return channel
}

// newPublishingChannel instantiates a new publishingChannel and amqpChannel for method inheritance.
//   - ctx is the parent context.
//   - connection is the parent amqp.Connection.
//   - keepAlive will keep the channel alive if true.
//   - retryDelay defines the delay between each retry, if the keepAlive flag is set to true.
//   - maxRetry defines the maximum number of times a message can be retried if its consumption failed.
//   - publishingCacheSize is the maximum cache size of failed publishing.
//   - publishingCacheTTL defines the time to live for each failed publishing that was put in cache.
//   - logger is the parent logger.
func newPublishingChannel(
	ctx context.Context,
	connection *amqp.Connection,
	keepAlive bool,
	retryDelay time.Duration,
	maxRetry uint,
	publishingCacheSize uint64,
	publishingCacheTTL time.Duration,
	logger logger,
) *amqpChannel {
	channel := &amqpChannel{
		ctx:        ctx,
		connection: connection,
		keepAlive:  keepAlive,
		retryDelay: retryDelay,
		logger: inheritLogger(logger, map[string]interface{}{
			"context": "channel",
			"type":    connectionTypePublisher,
		}),
		releaseLogger: &stdLogger{
			logger:     newLogrus(),
			identifier: libraryName,
			logFields: map[string]interface{}{
				"context": "channel",
				"type":    connectionTypePublisher,
			},
		},
		connectionType:  connectionTypePublisher,
		publishingCache: newTTLMap[string, mqttPublishing](publishingCacheSize, publishingCacheTTL),
		maxRetry:        maxRetry,
	}

	// We open an initial channel.
	err := channel.open()

	// If the channel failed to open and the keepAlive flag is set to true, we want to retry until success.
	if err != nil && keepAlive {
		go channel.retry()
	}

	return channel
}

// open opens a new amqp.Channel from the parent connection.
func (c *amqpChannel) open() error {
	// If the channel is nil or closed we return an error.
	if c.connection == nil || c.connection.IsClosed() {
		err := errConnectionClosed

		c.logger.Error(err, "Could not open channel")

		return err
	}

	// We request a channel from the parent connection.
	channel, err := c.connection.Channel()
	if err != nil {
		c.logger.Error(err, "Could not open channel")

		return err
	}

	c.channel = channel

	c.logger.Info("Channel opened")

	c.onChannelOpened()

	// If the keepAlive flag is set to true, we activate a new guard.
	if c.keepAlive {
		go c.guard()
	}

	return nil
}

// reconnect will indefinitely call the open method until a connection is successfully established or the context is canceled.
func (c *amqpChannel) retry() {
	c.logger.Debug("Retry launched")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Retry stopped by the context")

			// If the context was canceled, we break out of the method.
			return
		default:
			// Wait for the retryDelay.
			time.Sleep(c.retryDelay)

			// If there is no channel or the current channel is closed, we open a new channel.
			if !c.ready() {
				err := c.open()
				// If the operation succeeds, we break the loop.
				if err == nil {
					c.logger.Debug("Retry successful")

					return
				}

				c.logger.Error(err, "Could not open new channel during retry")
			} else {
				// If the channel exists and is active, we break out.
				return
			}
		}
	}
}

// guard is a channel safeguard that listens to channel close events and re-launches the channel.
func (c *amqpChannel) guard() {
	c.logger.Debug("Guard launched")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Guard stopped by the context")

			// If the context was canceled, we break out of the method.
			return
		case err, ok := <-c.channel.NotifyClose(make(chan *amqp.Error)):
			if !ok {
				return
			}

			if err != nil {
				c.logger.Warn("Channel lost", logField{Key: "reason", Value: err.Reason}, logField{Key: "code", Value: err.Code})
			}

			// If the channel was explicitly closed, we do not want to retry.
			if c.closed {
				return
			}

			c.onChannelClosed()

			go c.retry()

			return
		}
	}
}

// close the channel only if it is ready.
func (c *amqpChannel) close() error {
	if c.ready() {
		err := c.channel.Close()
		if err != nil {
			c.logger.Error(err, "Could not close channel")

			return err
		}
	}

	c.closed = true

	return nil
}

// ready returns true if the channel exists and is not closed.
func (c *amqpChannel) ready() bool {
	return c.channel != nil && !c.channel.IsClosed()
}

// healthy returns true if the channel exists and is not closed.
func (c *amqpChannel) healthy() bool {
	if c.connectionType == connectionTypeConsumer {
		return c.ready() && c.consumptionHealth.IsHealthy()
	}

	return c.ready()
}

// onChannelOpened is called when a channel is successfully opened.
func (c *amqpChannel) onChannelOpened() {
	if c.connectionType == connectionTypeConsumer {
		// We re-instantiate the consumptionContext and consumptionCancel.
		c.consumptionCtx, c.consumptionCancel = context.WithCancel(c.ctx)

		// This is just a safeguard.
		if c.consumer != nil {
			c.logger.Info("Launching consumer", logField{Key: "event", Value: "onChannelOpened"})

			// If the consumer is present we want to start consuming.
			go c.consume()
		}
	} else {
		// If the publishing cache is empty, nothing to do here.
		if c.publishingCache == nil || c.publishingCache.Len() == 0 {
			return
		}

		c.logger.Info("Emptying publishing cache", logField{Key: "event", Value: "onChannelOpened"})

		// For each cached unsuccessful message, we try publishing it again.
		c.publishingCache.ForEach(func(key string, msg mqttPublishing) {
			_ = c.channel.PublishWithContext(c.ctx, msg.Exchange, msg.RoutingKey, msg.Mandatory, msg.Immediate, msg.Msg)

			c.publishingCache.Delete(key)
		})
	}
}

// onChannelClosed is called when a channel is closed.
func (c *amqpChannel) onChannelClosed() {
	if c.connectionType == connectionTypeConsumer {
		c.logger.Info("Canceling consumptions", logField{Key: "event", Value: "onChannelClosed"})

		// We cancel the consumptionCtx.
		c.consumptionCancel()
	}
}

// getID returns a unique identifier for the channel.
func (c *amqpChannel) getID() string {
	if c.consumer == nil {
		return fmt.Sprintf("publisher_%s", uuid.NewString())
	}

	return fmt.Sprintf("%s_%s", c.consumer.Name, uuid.NewString())
}

// consume handles the consumption mechanism.
func (c *amqpChannel) consume() {
	// TODO(Alex): Check if this can actually happen
	// If the channel is not ready, we cannot consume.
	if !c.ready() {
		c.logger.Warn("Channel not ready, cannot launch consumer")

		return
	}

	// TODO(Alex): Double check why setting a prefetch size greater than 0 causes an error
	// Set the QOS, which defines how many messages can be processed at the same time.
	err := c.channel.Qos(c.consumer.PrefetchCount, c.consumer.PrefetchSize, false)
	if err != nil {
		c.logger.Error(err, "Could not define QOS for consumer")

		return
	}

	deliveries, err := c.channel.Consume(c.consumer.Queue, c.getID(), c.consumer.AutoAck, false, false, false, nil)

	c.consumptionHealth.AddSubscription(c.consumer.Queue, err)

	if err != nil {
		c.logger.Error(err, "Could not consume messages")

		// If the queue does not exist yet, we want to force a release log with a warning for better visibility.
		if isErrorNotFound(err) {
			c.releaseLogger.Warn("Queue does not exist", logField{Key: "queue", Value: c.consumer.Queue})
		}

		return
	}

	for {
		select {
		case <-c.consumptionCtx.Done():
			return
		case delivery := <-deliveries:
			// When a queue is deleted midway, a delivery with no tag or ID is received.
			if delivery.DeliveryTag == 0 && delivery.MessageId == "" {
				c.logger.Warn("Queue has been deleted, stopping consumer")

				return
			}

			// We copy the delivery for the concurrent process of it (otherwise we may process the wrong delivery
			// if a new one is consumed while the previous is still being processed).
			loopDelivery := delivery

			if c.consumer.ConcurrentProcess {
				// We process the message asynchronously if the concurrency is set to true.
				go c.processDelivery(&loopDelivery)
			} else {
				// Otherwise, we process the message synchronously.
				c.processDelivery(&loopDelivery)
			}
		}
	}
}

// processDelivery is the logic that defines what to do with a processed delivery and its error.
func (c *amqpChannel) processDelivery(delivery *amqp.Delivery) {
	handler := c.consumer.Handlers.FindFunc(delivery.RoutingKey)

	// If the handler doesn't exist for the received delivery, we negative acknowledge it without requeue.
	if handler == nil {
		c.logger.Debug("No handler found", logField{Key: "routingKey", Value: delivery.RoutingKey})

		// If the consumer is not set to auto acknowledge the delivery, we negative acknowledge it without requeue.
		if !c.consumer.AutoAck {
			_ = delivery.Nack(false, false)
		}

		return
	}

	err := handler(delivery.Body)

	// If the consumer has the autoAck flag activated, we want to retry the delivery in case of an error.
	if c.consumer.AutoAck {
		if err != nil {
			go c.retryDelivery(delivery, true)
		}

		return
	}

	// If there is no error, we can simply acknowledge the delivery.
	if err == nil {
		c.logger.Debug("Delivery successfully processed", logField{Key: "messageID", Value: delivery.MessageId})

		_ = delivery.Ack(false)

		return
	}

	// Otherwise we retry the delivery.
	go c.retryDelivery(delivery, false)
}

// retryDelivery processes a delivery retry based on its redelivery header.
//
//nolint:gocognit // We can allow the current complexity for now but we should revisit it later.
func (c *amqpChannel) retryDelivery(delivery *amqp.Delivery, alreadyAcknowledged bool) {
	c.logger.Debug("Delivery retry launched")

	for {
		select {
		case <-c.consumptionCtx.Done():
			c.logger.Debug("Delivery retry stopped by the consumption context")

			return
		default:
			// We wait for the retry delay before retrying a message.
			time.Sleep(c.retryDelay)

			// We first extract the xDeathCountHeader.
			maxRetryHeader, exists := delivery.Headers[xDeathCountHeader]

			// If the header doesn't exist.
			if !exists {
				c.logger.Debug("Delivery retry invalid")

				// We negative acknowledge the delivery without requeue if the autoAck flag is set to false.
				if !alreadyAcknowledged {
					_ = delivery.Nack(false, false)
				}

				return
			}

			// We then cast the value as an int32.
			retriesCount, ok := maxRetryHeader.(int32)

			// If the casting fails,we negative acknowledge the delivery without requeue if the autoAck flag is set to false.
			if !ok {
				c.logger.Debug("Delivery retry invalid")

				if !alreadyAcknowledged {
					_ = delivery.Nack(false, false)
				}

				return
			}

			// If the retries count is still greater than 0, we re-publish the delivery with a decremented xDeathCountHeader.
			if retriesCount > 0 {
				c.logger.Debug("Retrying delivery", logField{Key: "retriesLeft", Value: retriesCount - 1})

				// We first negative acknowledge the existing delivery to remove it from queue if the autoAck flag is set to false.
				if !alreadyAcknowledged {
					_ = delivery.Nack(false, false)
				}

				// We create a new publishing which is a copy of the old one but with a decremented xDeathCountHeader.
				newPublishing := amqp.Publishing{
					ContentType:  "application/json",
					Body:         delivery.Body,
					Type:         delivery.RoutingKey,
					Priority:     delivery.Priority,
					DeliveryMode: delivery.DeliveryMode,
					MessageId:    delivery.MessageId,
					Timestamp:    delivery.Timestamp,
					Headers: map[string]interface{}{
						xDeathCountHeader: int(retriesCount - 1),
					},
				}

				// We work on a best-effort basis. We try to re-publish the delivery, but we do nothing if it fails.
				_ = c.channel.PublishWithContext(c.ctx, delivery.Exchange, delivery.RoutingKey, false, false, newPublishing)

				return
			}

			c.logger.Debug("Cannot retry delivery, max retries reached")

			// Otherwise, we negative acknowledge the delivery without requeue if the autoAck flag is set to false.
			if !alreadyAcknowledged {
				_ = delivery.Nack(false, false)
			}

			return
		}
	}
}

// publish will publish a message with the given configuration.
func (c *amqpChannel) publish(exchange string, routingKey string, payload []byte, options *PublishingOptions) error {
	publishing := &amqp.Publishing{
		ContentType:  "application/json",
		Body:         payload,
		Type:         routingKey,
		Priority:     PriorityMedium.Uint8(),
		DeliveryMode: Persistent.Uint8(),
		MessageId:    uuid.NewString(),
		Timestamp:    time.Now(),
		Headers: map[string]interface{}{
			xDeathCountHeader: int(c.maxRetry),
		},
	}

	// If options are declared, we add the option.
	if options != nil {
		publishing.Priority = options.priority()
		publishing.DeliveryMode = options.mode()
	}

	// If the channel is not ready, we cannot publish, but we send the message to cache if the keepAlive flag is set to true.
	if !c.ready() {
		err := errChannelClosed

		if c.keepAlive {
			c.logger.Error(err, "Could not publish message, sending to cache")

			msg := mqttPublishing{
				Exchange:   exchange,
				RoutingKey: routingKey,
				Mandatory:  false,
				Immediate:  false,
				Msg:        *publishing,
			}

			c.publishingCache.Put(msg.HashCode(), msg)
		} else {
			c.logger.Error(err, "Could not publish message")
		}

		return err
	}

	err := c.channel.PublishWithContext(c.ctx, exchange, routingKey, false, false, *publishing)

	// If the message could not be sent we return an error without caching it.
	if err != nil {
		c.logger.Error(err, "Could not publish message")

		// If the exchange does not exist yet, we want to force a release log with a warning for better visibility.
		if isErrorNotFound(err) {
			c.releaseLogger.Warn(
				"The MQTT message was not sent, exchange does not exist",
				logField{Key: "exchange", Value: exchange},
				logField{Key: "routingKey", Value: routingKey},
			)
		}

		return err
	}

	c.logger.Debug("Message successfully sent", logField{Key: "messageID", Value: publishing.MessageId})

	return nil
}
