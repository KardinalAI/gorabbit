package gorabbit

import (
	"errors"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Error Utils.
const (
	codeNotFound = 404
)

// isErrorNotFound checks if the error returned by a connection or channel has the 404 code.
func isErrorNotFound(err error) bool {
	var amqpError *amqp.Error

	errors.As(err, &amqpError)

	if amqpError == nil {
		return false
	}

	return amqpError.Code == codeNotFound
}
