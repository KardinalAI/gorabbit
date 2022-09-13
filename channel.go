package gorabbit

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type amqpChannels []*amqpChannel

func (a amqpChannels) publishingChannel() *amqpChannel {
	for _, channel := range a {
		if channel != nil && channel.connectionType == Publisher {
			return channel
		}
	}

	return nil
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

	// activeGuard is an inner property that informs whether the guard was activated on the channel or not.
	activeGuard bool

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
	logger Logger

	// connectionType defines the ConnectionType.
	connectionType ConnectionType
}

// newConsumerChannel instantiates a new consumerChannel and amqpChannel for method inheritance.
// 	 - ctx is the parent context.
// 	 - connection is the parent amqp.Connection.
//   - keepAlive will keep the channel alive if true.
//   - retryDelay defines the delay between each retry, if the keepAlive flag is set to true.
//   - consumer is the MessageConsumer that will hold consumption information.
//   - maxRetry is the retry header for each message.
func newConsumerChannel(ctx context.Context, connection *amqp.Connection, keepAlive bool, retryDelay time.Duration, consumer *MessageConsumer, logger Logger) *amqpChannel {
	channel := newChannel(ctx, connection, keepAlive, retryDelay, logger, Consumer)

	channel.consumptionHealth = make(consumptionHealth)

	err := channel.registerConsumer(consumer)
	if err != nil {
		logger.Printf("Could not register consumer %s: %s", consumer.Name, err.Error())
	}

	return channel
}

// newPublishingChannel instantiates a new publishingChannel and amqpChannel for method inheritance.
// 	 - ctx is the parent context.
// 	 - connection is the parent amqp.Connection.
//   - keepAlive will keep the channel alive if true.
//   - retryDelay defines the delay between each retry, if the keepAlive flag is set to true.
//   - consumer is the MessageConsumer that will hold consumption information.
//   - maxRetry is the retry header for each message.
func newPublishingChannel(ctx context.Context, connection *amqp.Connection, keepAlive bool, retryDelay time.Duration, maxRetry uint, publishingCacheSize uint64, publishingCacheTTL time.Duration, logger Logger) *amqpChannel {
	channel := newChannel(ctx, connection, keepAlive, retryDelay, logger, Publisher)
	channel.publishingCache = newTTLMap[string, mqttPublishing](publishingCacheSize, publishingCacheTTL)
	channel.maxRetry = maxRetry

	return channel
}

// newChannel initializes a new amqpChannel with given arguments.
//   - ctx is the parent context.
//   - connection is the parent amqp.Connection.
//   - keepAlive will keep the channel alive if true.
//   - retryDelay defines the delay between each retry, if the keepAlive flag is set to true.
func newChannel(ctx context.Context, connection *amqp.Connection, keepAlive bool, retryDelay time.Duration, logger Logger, connectionType ConnectionType) *amqpChannel {
	channel := &amqpChannel{
		ctx:            ctx,
		connection:     connection,
		keepAlive:      keepAlive,
		retryDelay:     retryDelay,
		logger:         logger,
		connectionType: connectionType,
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
		return errConnectionClosed
	}

	// We request a channel from the parent connection.
	channel, err := c.connection.Channel()
	if err != nil {
		return err
	}

	c.channel = channel

	c.onChannelOpened()

	// If the keepAlive flag is set to true but no guard is active, we activate a new guard.
	if c.keepAlive && !c.activeGuard {
		go c.guard()
	}

	return nil
}

// reconnect will indefinitely call the open method until a connection is successfully established or the context is canceled.
func (c *amqpChannel) retry() {
	for {
		select {
		case <-c.ctx.Done():
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
					return
				}
			} else {
				// If the channel exists and is active, we break out.
				return
			}
		}
	}
}

// guard is a channel safeguard that listens to channel close events and re-launches the channel.
func (c *amqpChannel) guard() {
	c.activeGuard = true

	for {
		select {
		case <-c.ctx.Done():
			// If the context was canceled, we break out of the method.
			return
		case _, ok := <-c.channel.NotifyClose(make(chan *amqp.Error)):
			if !ok {
				return
			}

			// If the channel was explicitly closed, we do not want to retry.
			if c.closed {
				return
			}

			c.onChannelClosed()

			go c.retry()
		}
	}
}

// close the channel only if it is ready.
func (c *amqpChannel) close() error {
	if c.ready() {
		err := c.channel.Close()
		if err != nil {
			return err
		}

		c.closed = true

		return nil
	}

	return errChannelClosed
}

// ready returns true if the channel exists and is not closed.
func (c *amqpChannel) ready() bool {
	return c.channel != nil && !c.channel.IsClosed()
}

// healthy returns true if the channel exists and is not closed.
func (c *amqpChannel) healthy() bool {
	if c.connectionType == Consumer {
		return c.ready() && c.consumptionHealth.IsHealthy()
	}

	return c.ready()
}

// onChannelOpened is called when a channel is successfully opened.
func (c *amqpChannel) onChannelOpened() {
	if c.connectionType == Consumer {
		// We re-instantiate the consumptionContext and consumptionCancel.
		c.consumptionCtx, c.consumptionCancel = context.WithCancel(c.ctx)

		go c.consume()
	} else {
		// If the publishing cache is empty, nothing to do here.
		if c.publishingCache == nil || c.publishingCache.Len() == 0 {
			return
		}

		// For each cached unsuccessful message, we try publishing it again.
		c.publishingCache.ForEach(func(key string, msg mqttPublishing) {
			_ = c.channel.PublishWithContext(c.ctx, msg.Exchange, msg.RoutingKey, msg.Mandatory, msg.Immediate, msg.Msg)

			c.publishingCache.Delete(key)
		})
	}
}

// onChannelClosed is called when a channel is closed.
func (c *amqpChannel) onChannelClosed() {
	if c.connectionType == Consumer {
		// We cancel the consumptionCtx.
		c.consumptionCancel()
	}
}

// getID returns a unique identifier for the channel.
func (c *amqpChannel) getID() string {
	if c.consumer == nil {
		return fmt.Sprintf("%s", uuid.NewString())
	}

	return fmt.Sprintf("%s_%s", c.consumer.Name, uuid.NewString())
}

// registerConsumer registers the MessageConsumer for the channel and starts consuming if the channel is ready.
func (c *amqpChannel) registerConsumer(consumer *MessageConsumer) error {
	// If a consumer is already defined, we cannot overwrite it.
	if c.consumer != nil {
		return errConsumerExists
	}

	c.consumer = consumer

	// Set the QOS, which defines how many messages can be processed at the same time.
	err := c.channel.Qos(c.consumer.PrefetchCount, c.consumer.PrefetchSize, false)
	if err != nil {
	}

	// If the channel is ready, we can begin consumption.
	if c.ready() {
		go c.consume()
	}

	return nil
}

// consume handles the consumption mechanism.
func (c *amqpChannel) consume() {
	if c.consumer == nil {
		return
	}

	messages, err := c.channel.Consume(c.consumer.Queue, c.getID(), c.consumer.AutoAck, false, false, false, nil)

	c.consumptionHealth.AddSubscription(c.consumer.Queue, err)

	if err != nil {
		return
	}

	for {
		select {
		case <-c.consumptionCtx.Done():
			return
		case message := <-messages:
			// When a queue is deleted midway, a message with no delivery tag or ID is received.
			if message.DeliveryTag == 0 && message.MessageId == "" {
				return
			}

			if handler, found := c.consumer.Handlers[message.RoutingKey]; found {
				err = handler(message.Body)
			}
		}
	}
}

// publish will publish a message with the given configuration.
func (c *amqpChannel) publish(exchange string, routingKey string, payload []byte, options *publishingOptions) error {
	publishing := &amqp.Publishing{
		ContentType:  "text/plain",
		Body:         payload,
		Type:         routingKey,
		Priority:     PriorityMedium.Uint8(),
		DeliveryMode: Persistent.Uint8(),
		MessageId:    uuid.NewString(),
		Timestamp:    time.Now(),
		Headers: map[string]interface{}{
			MaxRetryHeader: int(c.maxRetry),
		},
	}

	// If options are declared, we add the option.
	if options != nil {
		publishing.Priority = options.priority()
		publishing.DeliveryMode = options.mode()
	}

	err := c.channel.PublishWithContext(c.ctx, exchange, routingKey, false, false, *publishing)

	// If the message could not be sent, we cache it until the channel is back up to publish it again.
	if err != nil {
		msg := mqttPublishing{
			Exchange:   exchange,
			RoutingKey: routingKey,
			Mandatory:  false,
			Immediate:  false,
			Msg:        *publishing,
		}

		c.publishingCache.Put(msg.HashCode(), msg)

		return err
	}

	return nil
}
