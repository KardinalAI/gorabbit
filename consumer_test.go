package gorabbit_test

import (
	"github.com/stretchr/testify/assert"
	"gitlab.kardinal.ai/coretech/gorabbit/v3"
	"log"
	"testing"
)

func TestMQTTMessageHandlers_FindFunc(t *testing.T) {
	handlers := gorabbit.MQTTMessageHandlers{
		"event.paola.#":            func(payload []byte) error { return nil },
		"event.phoebe.*.generated": func(payload []byte) error { return nil },
		"event.*.space.boom":       func(payload []byte) error { return nil },
		"*.toto.order.passed":      func(payload []byte) error { return nil },
		"#.toto":                   func(payload []byte) error { return nil },
	}

	tests := []struct {
		input       string
		shouldMatch bool
	}{
		{
			input:       "event.paola.plan.generated",
			shouldMatch: true,
		},
		{
			input:       "event.phoebe.order.generated",
			shouldMatch: true,
		},
		{
			input:       "event.phoebe.order.created",
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

	for i, test := range tests {
		log.Println("INDEX", i)
		fn := handlers.FindFunc(test.input)
		log.Println("FOUND FUNCTION", fn)

		if test.shouldMatch {
			assert.NotNil(t, fn)
		} else {
			assert.Nil(t, fn)
		}
	}
}
