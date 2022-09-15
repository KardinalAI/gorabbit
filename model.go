package gorabbit

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type ExchangeConfig struct {
	Name      string                 `yaml:"name"`
	Type      ExchangeType           `yaml:"type"`
	Persisted bool                   `yaml:"persisted"`
	Args      map[string]interface{} `yaml:"args"`
}

type QueueConfig struct {
	Name      string                 `yaml:"name"`
	Durable   bool                   `yaml:"durable"`
	Exclusive bool                   `yaml:"exclusive"`
	Args      map[string]interface{} `yaml:"args"`
	Bindings  *[]BindingConfig       `yaml:"bindings"`
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
