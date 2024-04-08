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

type PublishingOptions struct {
	MessagePriority *MessagePriority
	DeliveryMode    *DeliveryMode
}

func SendOptions() *PublishingOptions {
	return &PublishingOptions{}
}

func (m *PublishingOptions) priority() uint8 {
	if m.MessagePriority == nil {
		return PriorityMedium.Uint8()
	}

	return m.MessagePriority.Uint8()
}

func (m *PublishingOptions) mode() uint8 {
	if m.DeliveryMode == nil {
		return Persistent.Uint8()
	}

	return m.DeliveryMode.Uint8()
}

func (m *PublishingOptions) SetPriority(priority MessagePriority) *PublishingOptions {
	m.MessagePriority = &priority

	return m
}

func (m *PublishingOptions) SetMode(mode DeliveryMode) *PublishingOptions {
	m.DeliveryMode = &mode

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
