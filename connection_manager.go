package gorabbit

import (
	"context"
	"encoding/json"
	"time"
)

type connectionManager struct {
	// connections holds the publishing and consuming connections required.
	connections *amqpSaneConnections

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
	publishingCacheSize uint64,
	publishingCacheTTL time.Duration,
	logger Logger,
) *connectionManager {
	c := &connectionManager{
		connections: NewAMQPSaneConnections(ctx, uri, keepAlive, retryDelay, maxRetry, publishingCacheSize, publishingCacheTTL, logger),
	}

	return c
}

// close offers the basic connection and channel close() mechanism but with extra higher level checks.
func (c *connectionManager) close() error {
	return c.connections.close()
}

// isOperational returns true if both consumerConnection and publishingConnection are ready.
func (c *connectionManager) isOperational() bool {
	return c.connections.ready()
}

// isHealthy returns true if both consumerConnection and publishingConnection are healthy.
func (c *connectionManager) isHealthy() bool {
	return c.connections.healthy()
}

// registerConsumer registers a new MessageConsumer.
func (c *connectionManager) registerConsumer(consumer MessageConsumer) error {
	return c.connections.consumerConnection.registerConsumer(consumer)
}

func (c *connectionManager) publish(exchange, routingKey string, payload interface{}, options *publishingOptions) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return c.connections.publisherConnection.publish(exchange, routingKey, payloadBytes, options)
}

//// ExchangeDeclare does the same as the native amqp exchangeDeclare method, but checks for the connectionManager operational status first.
//func (c *connectionManager) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
//	if !c.isOperational() {
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
//func (c *connectionManager) ExchangeDelete(name string, ifUnused, noWait bool) error {
//	if !c.isOperational() {
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
//func (c *connectionManager) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
//	if !c.isOperational() {
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
//func (c *connectionManager) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
//	if !c.isOperational() {
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
//func (c *connectionManager) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
//	if !c.isOperational() {
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
//func (c *connectionManager) QueuePurge(name string, noWait bool) (int, error) {
//	if !c.isOperational() {
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
//func (c *connectionManager) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {
//	if !c.isOperational() {
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
//func (c *connectionManager) Get(queue string, autoAck bool) (amqp.Delivery, bool, error) {
//	if !c.isOperational() {
//		return amqp.Delivery{}, false, errConnectionOrChannelClosed
//	}
//
//	return c.channel.Get(queue, autoAck)
//}
