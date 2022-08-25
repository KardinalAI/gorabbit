package gorabbit

import (
	"errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"sync"
	"time"
)

type ClientConfig struct {
	Host      string
	Port      uint
	Username  string
	Password  string
	Mode      string // Mode is either release or debug (release by default)
	KeepAlive bool
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

// Deprecated: This is no longer used
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

	// If the message is already acknowledged then we just skip
	return nil
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

	// If the message is already not acknowledged then we just skip
	return nil
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

	// If the message is already rejected then we just skip
	return nil
}

func (msg *AMQPMessage) ToPublishing() amqp.Publishing {
	return amqp.Publishing{
		ContentType: msg.ContentType,
		Body:        msg.Body,
		Type:        msg.RoutingKey,
		Priority:    msg.Priority,
		MessageId:   msg.MessageId,
		Headers:     msg.Headers,
	}
}

type ttlMap struct {
	m map[uint64]time.Time
	l sync.Mutex
}

func newTTLMap(ln int, maxTTL time.Duration) (m *ttlMap) {
	m = &ttlMap{m: make(map[uint64]time.Time, ln)}
	go func() {
		for now := range time.Tick(maxTTL / 3) {
			m.l.Lock()
			for k, v := range m.m {
				if now.Sub(v) >= maxTTL {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
	return
}

func (m *ttlMap) Len() int {
	return len(m.m)
}

func (m *ttlMap) Put(k uint64) {
	m.l.Lock()
	defer m.l.Unlock()
	_, ok := m.m[k]
	if !ok {
		m.m[k] = time.Now()
	}
}

func (m *ttlMap) Get(k uint64) (v time.Time, found bool) {
	m.l.Lock()
	defer m.l.Unlock()
	v, found = m.m[k]
	return
}

type SubscriptionsHealth map[string]bool

func (s SubscriptionsHealth) IsHealthy() bool {
	for _, v := range s {
		if !v {
			return false
		}
	}
	return true
}

func (s SubscriptionsHealth) AddSubscription(queue string, err error) {
	if err != nil {
		s[queue] = false
	} else {
		s[queue] = true
	}
}
