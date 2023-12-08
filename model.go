package gorabbit

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type SchemaDefinitions struct {
	Exchanges []struct {
		Name       string `json:"name"`
		Vhost      string `json:"vhost"`
		Type       string `json:"type"`
		Durable    bool   `json:"durable"`
		AutoDelete bool   `json:"auto_delete"`
		Internal   bool   `json:"internal"`
		Arguments  struct {
		} `json:"arguments"`
	} `json:"exchanges"`
	Queues []struct {
		Name       string `json:"name"`
		Vhost      string `json:"vhost"`
		Durable    bool   `json:"durable"`
		AutoDelete bool   `json:"auto_delete"`
		Arguments  struct {
		} `json:"arguments"`
	} `json:"queues"`
	Bindings []struct {
		Source          string `json:"source"`
		Vhost           string `json:"vhost"`
		Destination     string `json:"destination"`
		DestinationType string `json:"destination_type"`
		RoutingKey      string `json:"routing_key"`
		Arguments       struct {
		} `json:"arguments"`
	} `json:"bindings"`
}

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
	Bindings  []BindingConfig        `yaml:"bindings"`
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

type RabbitMQEnvs struct {
	Host     string `env:"RABBITMQ_HOST"`
	Port     uint   `env:"RABBITMQ_PORT"`
	Username string `env:"RABBITMQ_USERNAME"`
	Password string `env:"RABBITMQ_PASSWORD"`
	Vhost    string `env:"RABBITMQ_VHOST"`
	UseTLS   bool   `env:"RABBITMQ_USE_TLS"`
}
