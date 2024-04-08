package gorabbit

import (
	"errors"
	"fmt"
	"strings"
)

// MQTTMessageHandlers is a wrapper that holds a map[string]MQTTMessageHandlerFunc.
type MQTTMessageHandlers map[string]MQTTMessageHandlerFunc

// MQTTMessageHandlerFunc is the function that will be called when a delivery is received.
type MQTTMessageHandlerFunc func(payload []byte) error

// Validate verifies that all routing keys in the handlers are properly formatted and allowed.
//
//nolint:gocognit // We can allow the current complexity for now but we should revisit it later.
func (mh MQTTMessageHandlers) Validate() error {
	for k := range mh {
		// A routing key cannot be empty.
		if len(k) == 0 {
			return errors.New("a routing key cannot be empty")
		}

		// A routing key cannot be equal to the wildcard '#'.
		if len(k) == 1 && k == "#" {
			return errors.New("a routing key cannot be the wildcard '#'")
		}

		// A routing key cannot contain spaces.
		if strings.Contains(k, " ") {
			return errors.New("a routing key cannot contain spaces")
		}

		// If a routing key is not just made up of one word.
		if strings.Contains(k, ".") {
			// We need to make sure that we do not find an empty word or a '%' in the middle of the key.
			split := strings.Split(k, ".")

			for i, v := range split {
				// We cannot have empty strings.
				if v == "" {
					return fmt.Errorf("the routing key '%s' is not properly formatted", k)
				}

				// The wildcard '#' is not allowed in the middle.
				if v == "#" && i > 0 && i < len(split)-1 {
					return fmt.Errorf("the wildcard '#' in the routing key '%s' is not allowed", k)
				}
			}
		}
	}

	return nil
}

// matchesPrefixWildcard verifies that everything that comes after the '#' wildcard matches.
func (mh MQTTMessageHandlers) matchesPrefixWildcard(storedWords, words []string) bool {
	// compareIndex starts after the wildcard in the storedWords array.
	compareIndex := 1

	// we initialize the wordIdx at -1.
	wordIdx := -1

	// Here we are searching for the first occurrence of the first word after the '#' wildcard
	// of the storedWords in the words.
	for i, w := range words {
		if w == storedWords[compareIndex] {
			// We can now start comparing at 'i'.
			wordIdx = i

			break
		}
	}

	// If we did not find the first word, then surely the key does not match.
	if wordIdx == -1 {
		return false
	}

	// If the length of storedWords is not the same as the length of words after the wildcard,
	// then surely the key does not match.
	if len(storedWords)-compareIndex != len(words)-wordIdx {
		return false
	}

	// Now we can compare, word by word if the routing keys matches.
	for i := wordIdx; i < len(words); i++ {
		// Be careful, if we find '*' then it should match no matter what.
		if storedWords[compareIndex] != words[i] && storedWords[compareIndex] != "*" {
			return false
		}

		// We move right in the storedWords.
		compareIndex++
	}

	return true
}

// matchesSuffixWildcard verifies that everything that comes before the '#' wildcard matches.
func (mh MQTTMessageHandlers) matchesSuffixWildcard(storedWords, words []string) bool {
	backCount := 2

	// compareIndex starts before the wildcard in the storedWords array.
	compareIndex := len(storedWords) - backCount

	// we initialize the wordIdx at -1.
	wordIdx := -1

	// Here we are searching for the first occurrence of the first word before the '#' wildcard
	// of the storedWords in the words.
	for i, w := range words {
		if w == storedWords[compareIndex] {
			wordIdx = i

			break
		}
	}

	// If we did not find the first word, then surely the key does not match.
	if wordIdx == -1 {
		return false
	}

	// If the indexes are not the same then surely the key does not match.
	if compareIndex != wordIdx {
		return false
	}

	// Now we can compare, word by word, going backwards if the routing keys matches.
	for i := wordIdx; i > -1; i-- {
		// Be careful, if we find '*' then it should match no matter what.
		if storedWords[compareIndex] != words[i] && storedWords[compareIndex] != "*" {
			return false
		}

		// We move left in the storedWords.
		compareIndex--
	}

	return true
}

// matchesSuffixWildcard verifies that 2 keys match word by word.
func (mh MQTTMessageHandlers) matchesKey(storedWords, words []string) bool {
	// If the lengths are not the same then surely the key does not match.
	if len(storedWords) != len(words) {
		return false
	}

	// Now we can compare, word by word if the routing keys matches.
	for i, word := range words {
		// Be careful, if we find '*' then it should match no matter what.
		if storedWords[i] != word && storedWords[i] != "*" {
			return false
		}
	}

	return true
}

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

		//nolint: gocritic,nestif // We need this if-else block
		if storedWords[0] == "#" {
			if !mh.matchesPrefixWildcard(storedWords, words) {
				continue
			}
		} else if storedWords[len(storedWords)-1] == "#" {
			if !mh.matchesSuffixWildcard(storedWords, words) {
				continue
			}
		} else {
			if !mh.matchesKey(storedWords, words) {
				continue
			}
		}

		return fn
	}

	// No matching keys were found.
	return nil
}

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
