package gorabbit

import (
	"fmt"
	"strings"
)

// MQTTMessageHandlers is a wrapper that holds a map[string]MQTTMessageHandlerFunc.
type MQTTMessageHandlers map[string]MQTTMessageHandlerFunc

func (mh MQTTMessageHandlers) FindFunc(routingKey string) MQTTMessageHandlerFunc {
	// We first check for a direct match
	if fn, found := mh[routingKey]; found {
		return fn
	}

	// Split the routing key into individual words.
	words := strings.Split(routingKey, ".")

	// Check if any of the registered keys match the routing key.
	for key, fn := range mh {
		// Split the registered key into individual words.
		storedWords := strings.Split(key, ".")

		match := true

		if len(words) < len(storedWords) && !strings.HasSuffix(key, "#") {
			match = false
			break
		}

		for i, word := range words {
			if word != "*" && word != "#" && storedWords[i] != word {
				match = false
				break
			}
		}

		if match {
			return fn
		}
	}

	// No matching keys were found.
	return nil
}

// MQTTMessageHandlerFunc is the function that will be called when a delivery is received.
type MQTTMessageHandlerFunc func(payload []byte) error

// MessageConsumer holds all the information needed to consume messages.
type MessageConsumer struct {
	// Queue defines the queue from which we want to consume messages.
	Queue string

	// Name is a unique identifier of the consumer. Should be as explicit as possible.
	Name string

	// PrefetchSize defines the max size of messages that are allowed to be processed at the same time.
	// This property is dropped if AutoAck is set to true.
	PrefetchSize int

	// PrefetchCount defines the max number of messages that are allowed to be processed at the same time.
	// This property is dropped if AutoAck is set to true.
	PrefetchCount int

	// AutoAck defines whether a message is directly acknowledged or not when being consumed.
	AutoAck bool

	// ConcurrentProcess will make MQTTMessageHandlers run concurrently for faster consumption, if set to true.
	ConcurrentProcess bool

	// Handlers is the list of defined handlers.
	Handlers MQTTMessageHandlers
}

// HashCode returns a unique identifier for the defined consumer.
func (c MessageConsumer) HashCode() string {
	return fmt.Sprintf("%s-%s", c.Queue, c.Name)
}
