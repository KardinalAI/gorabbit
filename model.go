package gorabbit

import (
	"github.com/streadway/amqp"
	"time"
)

type ClientConfig struct {
	Host       string
	Port       uint
	Username   string
	Password   string
	MaxRetry   uint
	RetryDelay time.Duration
}

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

type RabbitServerConfig struct {
	Exchanges []ExchangeConfig `yaml:"exchanges"`
	Queues    []QueueConfig    `yaml:"queues"`
}

type AMQPMessage struct {
	amqp.Delivery
	Type         string
	Microservice string
	Entity       string
	Action       string
}
