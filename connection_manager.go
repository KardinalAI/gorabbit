package gorabbit

import (
	"context"
	"encoding/json"
	"time"
)

type connectionManager struct {
	// consumerConnection holds the independent consuming connection.
	consumerConnection *amqpConnection

	// publisherConnection holds the independent publishing connection.
	publisherConnection *amqpConnection

	// logger is passed from the client for debugging purposes.
	logger logger
}

// newManager instantiates a new connectionManager with given arguments.
func newManager(
	ctx context.Context,
	uri string,
	keepAlive bool,
	retryDelay time.Duration,
	maxRetry uint,
	publishingCacheSize uint64,
	publishingCacheTTL time.Duration,
	logger logger,
) *connectionManager {
	c := &connectionManager{
		consumerConnection:  newConsumerConnection(ctx, uri, keepAlive, retryDelay, logger),
		publisherConnection: newPublishingConnection(ctx, uri, keepAlive, retryDelay, maxRetry, publishingCacheSize, publishingCacheTTL, logger),
	}

	return c
}

// close offers the basic connection and channel close() mechanism but with extra higher level checks.
func (c *connectionManager) close() error {
	if err := c.publisherConnection.close(); err != nil {
		return err
	}

	return c.consumerConnection.close()
}

// isReady returns true if both consumerConnection and publishingConnection are ready.
func (c *connectionManager) isReady() bool {
	if c.publisherConnection == nil || c.consumerConnection == nil {
		return false
	}

	return c.publisherConnection.ready() && c.consumerConnection.ready()
}

// isHealthy returns true if both consumerConnection and publishingConnection are healthy.
func (c *connectionManager) isHealthy() bool {
	if c.publisherConnection == nil || c.consumerConnection == nil {
		return false
	}

	return c.publisherConnection.healthy() && c.consumerConnection.healthy()
}

// registerConsumer registers a new MessageConsumer.
func (c *connectionManager) registerConsumer(consumer MessageConsumer) error {
	if c.consumerConnection == nil {
		return errConsumerConnectionNotInitialized
	}

	return c.consumerConnection.registerConsumer(consumer)
}

func (c *connectionManager) publish(exchange, routingKey string, payload interface{}, options *publishingOptions) error {
	if c.publisherConnection == nil {
		return errPublisherConnectionNotInitialized
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return c.publisherConnection.publish(exchange, routingKey, payloadBytes, options)
}

//// ExchangeDeclare does the same as the native amqp exchangeDeclare method, but checks for the connectionManager operational status first.
// func (c *connectionManager) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
//	if !c.isReady() {
//		return errConnectionOrChannelClosed
//	}
//
//	return c.channel.ExchangeDeclare(
//		name,
//		kind,
//		durable,
//		autoDelete,
//		internal,
//		noWait,
//		args,
//	)
//}
//
//// ExchangeDelete does the same as the native amqp exchangeDelete method, but checks for the connectionManager operational status first.
// func (c *connectionManager) ExchangeDelete(name string, ifUnused, noWait bool) error {
//	if !c.isReady() {
//		return errConnectionOrChannelClosed
//	}
//
//	return c.channel.ExchangeDelete(
//		name,
//		ifUnused,
//		noWait,
//	)
//}
//
//// QueueDeclare does the same as the native amqp queueDeclare method, but checks for the connectionManager operational status first.
// func (c *connectionManager) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
//	if !c.isReady() {
//		return amqp.Queue{}, errConnectionOrChannelClosed
//	}
//
//	return c.channel.QueueDeclare(
//		name,
//		durable,
//		autoDelete,
//		exclusive,
//		noWait,
//		args,
//	)
//}
//
//// QueueDeclarePassive does the same as the native amqp queueDeclarePassive method, but checks for the connectionManager operational status first.
// func (c *connectionManager) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
//	if !c.isReady() {
//		return amqp.Queue{}, errConnectionOrChannelClosed
//	}
//
//	return c.channel.QueueDeclarePassive(
//		name,
//		durable,
//		autoDelete,
//		exclusive,
//		noWait,
//		args,
//	)
//}
//
//// QueueBind does the same as the native amqp queueBind method, but checks for the connectionManager operational status first.
// func (c *connectionManager) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
//	if !c.isReady() {
//		return errConnectionOrChannelClosed
//	}
//
//	return c.channel.QueueBind(
//		name,
//		key,
//		exchange,
//		noWait,
//		args,
//	)
//}
//
//// QueuePurge does the same as the native amqp queuePurge method, but checks for the connectionManager operational status first.
// func (c *connectionManager) QueuePurge(name string, noWait bool) (int, error) {
//	if !c.isReady() {
//		return 0, errConnectionOrChannelClosed
//	}
//
//	return c.channel.QueuePurge(
//		name,
//		noWait,
//	)
//}
//
//// QueueDelete does the same as the native amqp queueDelete method, but checks for the connectionManager operational status first.
// func (c *connectionManager) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {
//	if !c.isReady() {
//		return 0, errConnectionOrChannelClosed
//	}
//
//	return c.channel.QueueDelete(
//		name,
//		ifUnused,
//		ifEmpty,
//		noWait,
//	)
//}
//
//// Get does the same as the native amqp get method, but checks for the connectionManager operational status first.
// func (c *connectionManager) Get(queue string, autoAck bool) (amqp.Delivery, bool, error) {
//	if !c.isReady() {
//		return amqp.Delivery{}, false, errConnectionOrChannelClosed
//	}
//
//	return c.channel.Get(queue, autoAck)
//}
