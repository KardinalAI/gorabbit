package gorabbit_test

import (
	"testing"

	"gitlab.kardinal.ai/coretech/gorabbit/v2"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func TestMessageTypeParsingSuccess(t *testing.T) {
	dummyType := "event.solange.solution.created"

	delivery := amqp.Delivery{
		Type: dummyType,
	}

	parsed, err := gorabbit.ParseMessage(delivery)

	assert.Nil(t, err)
	assert.Equal(t, parsed.Type, "event")
	assert.Equal(t, parsed.Microservice, "solange")
	assert.Equal(t, parsed.Entity, "solution")
	assert.Equal(t, parsed.Action, "created")
}

func TestMessageTypeParsingFailEmptyArg(t *testing.T) {
	dummyType := "event..solution."

	delivery := amqp.Delivery{
		Type: dummyType,
	}

	_, err := gorabbit.ParseMessage(delivery)

	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "empty argument")
}

func TestMessageTypeParsingFail(t *testing.T) {
	dummyType := "event.solange.solution"

	delivery := amqp.Delivery{
		Type: dummyType,
	}

	_, err := gorabbit.ParseMessage(delivery)

	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "invalid format")
}

func TestMessageTypeParsingEmpty(t *testing.T) {
	dummyType := ""

	delivery := amqp.Delivery{
		Type: dummyType,
	}

	_, err := gorabbit.ParseMessage(delivery)

	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "could not parse empty string")
}
