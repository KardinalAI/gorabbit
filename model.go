package gorabbit

import (
	"errors"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
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

func (msg *AMQPMessage) GetRedeliveryCount() int {
	val, ok := msg.Headers[RedeliveryHeader]

	if !ok {
		return 0
	} else {
		return int(val.(int32))
	}
}

func (msg *AMQPMessage) IncrementRedeliveryHeader() int {
	redeliveredCount := msg.GetRedeliveryCount()
	redeliveredCount++

	newHeader := map[string]interface{}{
		RedeliveryHeader: redeliveredCount,
	}

	msg.Headers = newHeader

	return redeliveredCount
}

func (msg *AMQPMessage) Ack(multiple bool) error {
	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		if msg.Acknowledger != nil {
			err := msg.Acknowledger.Ack(msg.DeliveryTag, multiple)
			if err != nil {
				return err
			}
			consumed.Put(msg.DeliveryTag)
			return nil
		} else {
			return errors.New("delivery not initialized")
		}
	}

	return fmt.Errorf("message %d already acknowledged", msg.DeliveryTag)
}

func (msg *AMQPMessage) Nack(multiple bool, requeue bool) error {
	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		if msg.Acknowledger != nil {
			err := msg.Acknowledger.Nack(msg.DeliveryTag, multiple, requeue)
			if err != nil {
				return err
			}
			consumed.Put(msg.DeliveryTag)
			return nil
		} else {
			return errors.New("delivery not initialized")
		}
	}

	return fmt.Errorf("message %d already not acknowledged", msg.DeliveryTag)
}

func (msg *AMQPMessage) Reject(requeue bool) error {
	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		if msg.Acknowledger != nil {
			err := msg.Acknowledger.Reject(msg.DeliveryTag, requeue)
			if err != nil {
				return err
			}
			consumed.Put(msg.DeliveryTag)
			return nil
		} else {
			return errors.New("delivery not initialized")
		}
	}

	return fmt.Errorf("message %d already rejected", msg.DeliveryTag)
}
