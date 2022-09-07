package gorabbit

import "fmt"

type MQTTMessageHandlers map[string]func(payload []byte) error

type MessageRedirection struct {
	From     string
	To       string
	Exchange string
}

type MessageConsumer struct {
	Queue        string
	Consumer     string
	AutoAck      bool
	Handlers     MQTTMessageHandlers
	Redirections []MessageRedirection
	OnBadFormat  func(string, []byte)
	OnRetryError func(string, error)
}

func (c MessageConsumer) HashCode() string {
	return fmt.Sprintf("%s-%s", c.Queue, c.Consumer)
}
