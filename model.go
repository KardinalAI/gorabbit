package gorabbit

import (
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ExchangeConfig struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`
	Persisted bool   `yaml:"persisted"`
}

type QueueConfig struct {
	Name      string           `yaml:"name"`
	Durable   bool             `yaml:"durable"`
	Exclusive bool             `yaml:"exclusive"`
	Bindings  *[]BindingConfig `yaml:"bindings"`
}

type BindingConfig struct {
	RoutingKey string `yaml:"routing_key"`
	Exchange   string `yaml:"exchange"`
}

type publishingOptions struct {
	messagePriority *MessagePriority
	deliveryMode    *DeliveryMode
}

func SendOptions() *publishingOptions {
	return &publishingOptions{}
}

func (m *publishingOptions) priority() uint8 {
	if m.messagePriority == nil {
		return PriorityMedium.Uint8()
	}

	return m.messagePriority.Uint8()
}

func (m *publishingOptions) mode() uint8 {
	if m.deliveryMode == nil {
		return Persistent.Uint8()
	}

	return m.deliveryMode.Uint8()
}

func (m *publishingOptions) SetPriority(priority MessagePriority) *publishingOptions {
	m.messagePriority = &priority

	return m
}

func (m *publishingOptions) SetMode(mode DeliveryMode) *publishingOptions {
	m.deliveryMode = &mode

	return m
}

type amqpMessage struct {
	amqp.Delivery
	Type         string
	Microservice string
	Entity       string
	Action       string
}

func (msg *amqpMessage) GetRedeliveryCount() int32 {
	val, ok := msg.Headers[RedeliveryHeader]

	if !ok {
		return 0
	} else {
		return val.(int32)
	}
}

func (msg *amqpMessage) IncrementRedeliveryHeader() int32 {
	redeliveredCount := msg.GetRedeliveryCount()
	redeliveredCount++

	newHeader := map[string]interface{}{
		RedeliveryHeader: redeliveredCount,
	}

	msg.Headers = newHeader

	return redeliveredCount
}

func (msg *amqpMessage) Ack(multiple bool) error {
	if msg.Acknowledger == nil {
		return errDeliveryNotInitialized
	}

	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		err := msg.Acknowledger.Ack(msg.DeliveryTag, multiple)
		if err != nil {
			return err
		}

		consumed.Put(msg.DeliveryTag, nil)

		return nil
	}

	// If the message is already acknowledged then we just skip
	return nil
}

func (msg *amqpMessage) Nack(multiple bool, requeue bool) error {
	if msg.Acknowledger == nil {
		return errDeliveryNotInitialized
	}

	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		err := msg.Acknowledger.Nack(msg.DeliveryTag, multiple, requeue)
		if err != nil {
			return err
		}

		consumed.Put(msg.DeliveryTag, nil)

		return nil
	}

	// If the message is already not acknowledged then we just skip
	return nil
}

func (msg *amqpMessage) Reject(requeue bool) error {
	if msg.Acknowledger == nil {
		return errDeliveryNotInitialized
	}

	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		err := msg.Acknowledger.Reject(msg.DeliveryTag, requeue)
		if err != nil {
			return err
		}

		consumed.Put(msg.DeliveryTag, nil)

		return nil
	}

	// If the message is already rejected then we just skip
	return nil
}

func (msg *amqpMessage) ToPublishing() amqp.Publishing {
	return amqp.Publishing{
		ContentType: msg.ContentType,
		Body:        msg.Body,
		Type:        msg.RoutingKey,
		Priority:    msg.Priority,
		MessageId:   msg.MessageId,
		Headers:     msg.Headers,
	}
}

type consumptionHealth map[string]bool

func (s consumptionHealth) IsHealthy() bool {
	for _, v := range s {
		if !v {
			return false
		}
	}

	return true
}

func (s consumptionHealth) AddSubscription(queue string, err error) {
	s[queue] = err == nil
}

type mqttPublishing struct {
	Exchange   string
	RoutingKey string
	Mandatory  bool
	Immediate  bool
	Msg        amqp.Publishing
}

func (m mqttPublishing) HashCode() string {
	return m.Msg.MessageId
}

// ParseMessage takes an amqp.Deliver as argument, extracts the routingKey from the Type, and parses it as a amqpMessage.
// This method is purely for Kardinal's needs and should be removed in the library is made public.
func ParseMessage(delivery amqp.Delivery) (*amqpMessage, error) {
	messageArgs := delivery.Type

	if messageArgs == "" {
		return nil, errEmptyStringParse
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

	return &amqpMessage{
		Delivery:     delivery,
		Type:         splitArgs[0],
		Microservice: splitArgs[1],
		Entity:       splitArgs[2],
		Action:       splitArgs[3],
	}, nil
}
