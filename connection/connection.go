package connection

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

	// activeGuard is an inner property that informs whether the guard was activated on the connection or not.
	activeGuard bool

	// closed is an inner property that switches to true if the connection was explicitly closed.
	closed bool
}

// New initializes a new amqpConnection with given arguments.
//   - ctx is the parent context.
//   - uri is the connection string.
//   - keepAlive will keep the connection alive if true.
//   - retryDelay defines the delay between each re-connection, if the keepAlive flag is set to true.
func New(ctx context.Context, uri string, keepAlive bool, retryDelay time.Duration) *amqpConnection {
	conn := &amqpConnection{
		ctx:        ctx,
		uri:        uri,
		keepAlive:  keepAlive,
		retryDelay: retryDelay,
	}

	// We open an initial connection.
	err := conn.open()

	// If the connection failed and the keepAlive flag is set to true, we want to re-connect until success.
	if err != nil && keepAlive {
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

	// If the keepAlive flag is set to true but no guard is active, we activate a new guard.
	if a.keepAlive && !a.activeGuard {
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
			if !a.Ready() {
				err := a.open()
				// If the operation succeeds, we break the loop.
				if err == nil {
					return
				}
			} else {
				// If the connection exists and is active, we break out.
				return
			}
		}
	}
}

// guard is a connection safeguard that listens to connection close events and re-launches the connection.
func (a *amqpConnection) guard() {
	a.activeGuard = true

	for {
		select {
		case <-a.ctx.Done():
			// If the context was canceled, we break out of the method.
			return
		case _, ok := <-a.connection.NotifyClose(make(chan *amqp.Error)):
			if !ok {
				return
			}

			// If the connection was explicitly closed, we do not want to re-connect.
			if a.closed {
				return
			}

			go a.reconnect()
		}
	}
}

// Close closes the connection only if it is Ready.
func (a *amqpConnection) Close() {
	if a.Ready() {
		err := a.connection.Close()
		if err == nil {
			a.closed = true
		}
	}
}

// Ready returns true of the connection exists and is not closed.
func (a *amqpConnection) Ready() bool {
	return a.connection != nil && !a.connection.IsClosed()
}
