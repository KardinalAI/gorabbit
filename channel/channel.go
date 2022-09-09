package channel

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

	// closed is an inner property that switches to true if the channel was explicitly closed.
	closed bool
}

// New initializes a new amqpChannel with given arguments.
//   - ctx is the parent context.
//   - connection is the parent amqp.Connection.
//   - keepAlive will keep the channel alive if true.
//   - retryDelay defines the delay between each retry, if the keepAlive flag is set to true.
func New(ctx context.Context, connection *amqp.Connection, keepAlive bool, retryDelay time.Duration) *amqpChannel {
	channel := &amqpChannel{
		ctx:        ctx,
		connection: connection,
		keepAlive:  keepAlive,
		retryDelay: retryDelay,
	}

	// We open an initial channel.
	err := channel.open()

	// If the channel failed to open and the keepAlive flag is set to true, we want to retry until success.
	if err != nil && keepAlive {
		go channel.retry()
	}

	return channel
}

// open opens a new amqp.Connection with the help of a defined uri.
func (c *amqpChannel) open() error {
	// If the connection is nil or closed we return an error.
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
			if !c.Ready() {
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

// Close closes the channel only if it is Ready.
func (c *amqpChannel) Close() {
	if c.Ready() {
		err := c.connection.Close()
		if err == nil {
			c.closed = true
		}
	}
}

// Ready returns true if the channel exists and is not closed.
func (c *amqpChannel) Ready() bool {
	return c.channel != nil && !c.channel.IsClosed()
}

// onChannelOpened is called when a channel is successfully opened.
func (c *amqpChannel) onChannelOpened() {}

// onChannelClosed is called when a channel is closed.
func (c *amqpChannel) onChannelClosed() {}
