package v2

import (
	"errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"strings"
)

func ParseMessage(delivery amqp.Delivery) (*AMQPMessage, error) {
	messageArgs := delivery.Type

	if messageArgs == "" {
		return nil, errors.New("could not parse empty string")
	}

	splitArgs := strings.Split(messageArgs, ".")

	if len(splitArgs) < 4 {
		return nil, errors.New("invalid format")
	}

	for _, arg := range splitArgs {
		if arg == "" {
			return nil, errors.New("empty argument")
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
