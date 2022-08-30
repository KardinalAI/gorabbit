package gorabbit

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionManager struct {
	uri                       string
	ctx                       context.Context
	connection                *amqp.Connection
	channel                   *amqp.Channel
	connectionStatus          chan ConnectionStatus
	onConnectionStatusChanged func(status ConnectionStatus)
	keepAlive                 bool
	subscriptions             SubscriptionsHealth
	logger                    Logger
}

func newManager(ctx context.Context, uri string, keepAlive bool, onConnectionStatusChanged func(status ConnectionStatus), logger Logger) *connectionManager {
	c := &connectionManager{
		uri:                       uri,
		ctx:                       ctx,
		connectionStatus:          make(chan ConnectionStatus),
		onConnectionStatusChanged: onConnectionStatusChanged,
		keepAlive:                 keepAlive,
		subscriptions:             make(SubscriptionsHealth),
		logger:                    logger,
	}

	if !c.keepAlive {
		if err := c.newConnection(); err != nil {
			c.logger.Printf("could not create new connection: %s", err.Error())
			c.registerStatusChange(ConnFailed)
		}

		return c
	}

	go func() {
		for {
			err := c.newConnection()
			if err != nil {
				c.logger.Printf("could not create new connection: %s", err.Error())
				c.registerStatusChange(ConnFailed)
				time.Sleep(reconnectDelay)
			} else {
				break
			}
		}
	}()

	return c
}

func (c *connectionManager) newConnection() error {
	if c.uri == "" {
		return errEmptyURI
	}

	if c.connection == nil || c.connection.IsClosed() {
		conn, err := amqp.Dial(c.uri)

		if err != nil {
			return err
		}

		c.registerStatusChange(ConnUp)

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

	c.registerStatusChange(ChanUp)

	c.channel = ch

	const prefetchCount = 10

	const prefetchSize = 0

	err = c.channel.Qos(prefetchCount, prefetchSize, false)

	if err != nil {
		c.logger.Printf("could not declare QOS with prefetch count of %d", prefetchCount)
		return err
	}

	if c.keepAlive {
		go c.keepChannelAlive()
	}

	return nil
}

func (c *connectionManager) registerStatusChange(status ConnectionStatus) {
	if c.onConnectionStatusChanged != nil {
		c.onConnectionStatusChanged(status)
	} else {
		c.connectionStatus <- status
	}
}

func (c *connectionManager) close() error {
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
				// wait reconnectDelay before trying to reconnect
				time.Sleep(reconnectDelay)

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

func (c *connectionManager) keepChannelAlive() {
	for {
		c.logger.Printf("Trying to create a new channel")
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
				// wait reconnectDelay before trying to reconnect
				time.Sleep(reconnectDelay)

				select {
				case <-c.ctx.Done():
					c.logger.Printf("Cannot keep channel alive because the context is done")
					return
				default:
					if c.connection == nil || c.connection.IsClosed() {
						c.logger.Printf("Cannot create new channel without a connection")
						continue
					}

					var err error
					c.channel, err = c.connection.Channel()
					if err == nil {
						c.registerStatusChange(ChanUp)
						break Loop
					}
				}
			}
		}
	}
}

func (c *connectionManager) Publish(exchange, routingKey string, mandatory, immediate bool, msg amqp.Publishing) error {
	if !c.isOperational() {
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

func (c *connectionManager) QueuePurge(name string, noWait bool) (int, error) {
	if !c.isOperational() {
		return 0, errConnectionOrChannelClosed
	}

	return c.channel.QueuePurge(
		name,
		noWait,
	)
}

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

func (c *connectionManager) Get(queue string, autoAck bool) (amqp.Delivery, bool, error) {
	if !c.isOperational() {
		return amqp.Delivery{}, false, errConnectionOrChannelClosed
	}

	return c.channel.Get(queue, autoAck)
}
