package gorabbit

import (
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ClientConfig struct {
	Host      string
	Port      uint
	Username  string
	Password  string
	KeepAlive bool
	Mode      string // Mode is either release or debug (release by default)
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

type sendOptions struct {
	messagePriority *MessagePriority
	deliveryMode    *DeliveryMode
}

func SendOptions() *sendOptions {
	return &sendOptions{}
}

func (m *sendOptions) priority() uint8 {
	if m.messagePriority == nil {
		return PriorityMedium.Uint8()
	}

	return m.messagePriority.Uint8()
}

func (m *sendOptions) mode() uint8 {
	if m.deliveryMode == nil {
		return Persistent.Uint8()
	}

	return m.deliveryMode.Uint8()
}

func (m *sendOptions) SetPriority(priority MessagePriority) *sendOptions {
	m.messagePriority = &priority

	return m
}

func (m *sendOptions) SetMode(mode DeliveryMode) *sendOptions {
	m.deliveryMode = &mode

	return m
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
	if msg.Acknowledger == nil {
		return errDeliveryNotInitialized
	}

	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		err := msg.Acknowledger.Ack(msg.DeliveryTag, multiple)
		if err != nil {
			return err
		}

		consumed.Put(msg.DeliveryTag)

		return nil
	}

	// If the message is already acknowledged then we just skip
	return nil
}

func (msg *AMQPMessage) Nack(multiple bool, requeue bool) error {
	if msg.Acknowledger == nil {
		return errDeliveryNotInitialized
	}

	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		err := msg.Acknowledger.Nack(msg.DeliveryTag, multiple, requeue)
		if err != nil {
			return err
		}

		consumed.Put(msg.DeliveryTag)

		return nil
	}

	// If the message is already not acknowledged then we just skip
	return nil
}

func (msg *AMQPMessage) Reject(requeue bool) error {
	if msg.Acknowledger == nil {
		return errDeliveryNotInitialized
	}

	if _, ok := consumed.Get(msg.DeliveryTag); !ok {
		err := msg.Acknowledger.Reject(msg.DeliveryTag, requeue)
		if err != nil {
			return err
		}

		consumed.Put(msg.DeliveryTag)

		return nil
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

func newTTLMap(ln int, maxTTL time.Duration) *ttlMap {
	m := &ttlMap{m: make(map[uint64]time.Time, ln)}

	go func() {
		const tickFraction = 3

		for now := range time.Tick(maxTTL / tickFraction) {
			m.l.Lock()
			for k, v := range m.m {
				if now.Sub(v) >= maxTTL {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()

	return m
}

func (m *ttlMap) Len() int {
	return len(m.m)
}

func (m *ttlMap) Put(k uint64) {
	m.l.Lock()

	defer m.l.Unlock()

	if _, ok := m.m[k]; !ok {
		m.m[k] = time.Now()
	}
}

func (m *ttlMap) Get(k uint64) (time.Time, bool) {
	m.l.Lock()

	defer m.l.Unlock()

	v, found := m.m[k]

	return v, found
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
