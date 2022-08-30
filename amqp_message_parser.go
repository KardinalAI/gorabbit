package gorabbit

import (
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

func ParseMessage(delivery amqp.Delivery) (*AMQPMessage, error) {
	messageArgs := delivery.Type

	if messageArgs == "" {
		return nil, errStringParse
	}

	splitArgs := strings.Split(messageArgs, ".")

	const expectedArgsLength = 4

	if len(splitArgs) < expectedArgsLength {
		return nil, errInvalidFormat
	}

	for _, arg := range splitArgs {
		if arg == "" {
			return nil, errEmptyArgument
		}
	}

	return &AMQPMessage{
		Delivery:     delivery,
		Type:         splitArgs[0],
		Microservice: splitArgs[1],
		Entity:       splitArgs[2],
		Action:       splitArgs[3],
	}, nil
}
