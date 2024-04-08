package gorabbit_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KardinalAI/gorabbit"
)

func TestMQTTMessageHandlers_Validate(t *testing.T) {
	tests := []struct {
		handlers      gorabbit.MQTTMessageHandlers
		expectedError error
	}{
		{
			handlers: gorabbit.MQTTMessageHandlers{
				"event.user.#":            func(_ []byte) error { return nil },
				"event.email.*.generated": func(_ []byte) error { return nil },
				"event.*.space.boom":      func(_ []byte) error { return nil },
				"*.toto.order.passed":     func(_ []byte) error { return nil },
				"#.toto":                  func(_ []byte) error { return nil },
			},
			expectedError: nil,
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				"": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("a routing key cannot be empty"),
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				" ": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("a routing key cannot contain spaces"),
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				"#": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("a routing key cannot be the wildcard '#'"),
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				"toto.#.titi": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("the wildcard '#' in the routing key 'toto.#.titi' is not allowed"),
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				"toto titi": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("a routing key cannot contain spaces"),
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				"toto..titi": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("the routing key 'toto..titi' is not properly formatted"),
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				".toto.titi": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("the routing key '.toto.titi' is not properly formatted"),
		},
		{
			handlers: gorabbit.MQTTMessageHandlers{
				"toto.titi.": func(_ []byte) error { return nil },
			},
			expectedError: errors.New("the routing key 'toto.titi.' is not properly formatted"),
		},
	}

	for _, test := range tests {
		err := test.handlers.Validate()

		assert.Equal(t, test.expectedError, err)
	}
}

func TestMQTTMessageHandlers_FindFunc(t *testing.T) {
	handlers := gorabbit.MQTTMessageHandlers{
		"event.user.#":            func(_ []byte) error { return nil },
		"event.email.*.generated": func(_ []byte) error { return nil },
		"event.*.space.boom":      func(_ []byte) error { return nil },
		"*.toto.order.passed":     func(_ []byte) error { return nil },
		"#.toto":                  func(_ []byte) error { return nil },
	}

	tests := []struct {
		input       string
		shouldMatch bool
	}{
		{
			input:       "event.user.plan.generated",
			shouldMatch: true,
		},
		{
			input:       "event.user.password.generated.before.awakening.the.titan",
			shouldMatch: true,
		},
		{
			input:       "event.email.subject.generated",
			shouldMatch: true,
		},
		{
			input:       "event.email.toto.generated",
			shouldMatch: true,
		},
		{
			input:       "event.email.titi.generated",
			shouldMatch: true,
		},
		{
			input:       "event.email.order.created",
			shouldMatch: false,
		},
		{
			input:       "event.toto.space.boom",
			shouldMatch: true,
		},
		{
			input:       "event.toto.space.not_boom",
			shouldMatch: false,
		},
		{
			input:       "command.toto.order.passed",
			shouldMatch: true,
		},
		{
			input:       "command.toto.order.passed.please",
			shouldMatch: false,
		},
		{
			input:       "event.toto",
			shouldMatch: true,
		},
		{
			input:       "event.space.space.toto",
			shouldMatch: true,
		},
		{
			input:       "event.toto.space",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		fn := handlers.FindFunc(test.input)

		if test.shouldMatch {
			assert.NotNil(t, fn)
		} else {
			assert.Nil(t, fn)
		}
	}
}
