package gorabbit

import (
	"context"
	"errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

type connectionManager struct {
	uri              string
	ctx              context.Context
	connection       *amqp.Connection
	channel          *amqp.Channel
	connectionStatus chan ConnectionStatus
	keepAlive        bool
	subscriptions    SubscriptionsHealth
	logger           Logger
}

func newManager(ctx context.Context, uri string, keepAlive bool, logger Logger) *connectionManager {
	c := &connectionManager{
		uri:              uri,
		ctx:              ctx,
		connectionStatus: make(chan ConnectionStatus, 5),
		keepAlive:        keepAlive,
		subscriptions:    make(SubscriptionsHealth),
		logger:           logger,
	}

	if c.keepAlive {
		go func() {
			for {
				err := c.newConnection()
				if err != nil {
					c.logger.Printf("could not create new connection: %s", err.Error())
					c.connectionStatus <- ConnFailed
					time.Sleep(reconnectDelay)
				} else {
					break
				}
			}
		}()
	} else {
		err := c.newConnection()
		if err != nil {
			c.logger.Printf("could not create new connection: %s", err.Error())
			c.connectionStatus <- ConnFailed
		}
	}

	return c
}

var connectionError = errors.New("connection or channel closed")

func (c *connectionManager) newConnection() error {
	if c.uri == "" {
		return errors.New("amqp uri is empty")
	}

	if c.connection == nil || c.connection.IsClosed() {
		conn, err := amqp.Dial(c.uri)

		if err != nil {
			return err
		}

		c.connectionStatus <- ConnUp

		c.connection = conn

		if c.keepAlive {
			go c.keepConnectionAlive()
		}
	}

	return c.newChannel()
}

func (c *connectionManager) newChannel() error {
	ch, err := c.connection.Channel()

	if err != nil {
		return err
	}

	c.connectionStatus <- ChanUp

	c.channel = ch

	err = c.channel.Qos(10, 0, false)

	if err != nil {
		c.logger.Printf("could not declare QOS with prefetch count of %d", 10)
		return err
	}

	if c.keepAlive {
		go c.keepChannelAlive()
	}

	return nil
}

func (c *connectionManager) close() error {
	if c.channel != nil && !c.channel.IsClosed() {
		err := c.channel.Close()

		if err != nil {
			return err
		}

		if !c.keepAlive {
			c.connectionStatus <- ChanDown
		}

		c.channel = nil
	}

	if c.connection != nil && !c.connection.IsClosed() {
		err := c.connection.Close()

		if err != nil {
			return err
		}

		if !c.keepAlive {
			c.connectionStatus <- ConnDown
		}
	}

	c.connection = nil

	return nil
}

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

func (c *connectionManager) isHealthy() bool {
	return c.subscriptions.IsHealthy()
}

func (c *connectionManager) keepConnectionAlive() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case _, ok := <-c.connection.NotifyClose(make(chan *amqp.Error)):
			c.connectionStatus <- ConnDown
			if !ok {
				return
			}

		Loop:
			for {
				// wait reconnectDelay before trying to reconnect
				time.Sleep(reconnectDelay)

				select {
				case <-c.ctx.Done():
					return
				default:
					var err error
					c.connection, err = amqp.Dial(c.uri)
					if err == nil {
						c.connectionStatus <- ConnUp
						break Loop
					}
				}
			}
		}
	}
}

func (c *connectionManager) keepChannelAlive() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case _, ok := <-c.channel.NotifyClose(make(chan *amqp.Error)):
			c.connectionStatus <- ChanDown

			if !ok {
				return
			}

		Loop:
			for {
				// wait reconnectDelay before trying to reconnect
				time.Sleep(reconnectDelay)

				select {
				case <-c.ctx.Done():
					return
				default:
					if c.connection == nil || c.connection.IsClosed() {
						continue
					}

					var err error
					c.channel, err = c.connection.Channel()
					if err == nil {
						c.connectionStatus <- ChanUp
						break Loop
					}
				}
			}
		}
	}
}

func (c *connectionManager) Publish(exchange, routingKey string, mandatory, immediate bool, msg amqp.Publishing, opts ...*MessageSendOptions) error {
	if !c.isOperational() {
		return connectionError
	}

	err := c.channel.PublishWithContext(
		c.ctx,
		exchange,
		routingKey,
		mandatory,
		immediate,
		msg,
	)

	shouldRetry := opts != nil && opts[0] != nil && opts[0].GetRetryCount() > 0

	if err != nil && shouldRetry {
		go func() {
			retryCount := opts[0].GetRetryCount()

			c.logger.Printf("Trying to send message again in %d seconds", reconnectDelay)

			time.Sleep(reconnectDelay)

			newOpts := SendOptions().SetRetryCount(retryCount - 1)

			_ = c.Publish(exchange, routingKey, mandatory, immediate, msg, newOpts)
		}()
	}

	return err
}

func (c *connectionManager) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	if !c.isOperational() {
		return nil, connectionError
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

func (c *connectionManager) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	if !c.isOperational() {
		return connectionError
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

func (c *connectionManager) ExchangeDelete(name string, ifUnused, noWait bool) error {
	if !c.isOperational() {
		return connectionError
	}

	return c.channel.ExchangeDelete(
		name,
		ifUnused,
		noWait,
	)
}

func (c *connectionManager) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if !c.isOperational() {
		return amqp.Queue{}, connectionError
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

func (c *connectionManager) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if !c.isOperational() {
		return amqp.Queue{}, connectionError
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

func (c *connectionManager) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	if !c.isOperational() {
		return connectionError
	}

	return c.channel.QueueBind(
		name,
		key,
		exchange,
		noWait,
		args,
	)
}

func (c *connectionManager) QueuePurge(name string, noWait bool) (int, error) {
	if !c.isOperational() {
		return 0, connectionError
	}

	return c.channel.QueuePurge(
		name,
		noWait,
	)
}

func (c *connectionManager) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {
	if !c.isOperational() {
		return 0, connectionError
	}

	return c.channel.QueueDelete(
		name,
		ifUnused,
		ifEmpty,
		noWait,
	)
}

func (c *connectionManager) Get(queue string, autoAck bool) (amqp.Delivery, bool, error) {
	if !c.isOperational() {
		return amqp.Delivery{}, false, connectionError
	}

	return c.channel.Get(queue, autoAck)
}
