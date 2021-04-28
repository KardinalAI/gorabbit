package gorabbit

// Topic Type of exchanges
// (Only topic supported for now)
const (
	Topic = "topic"
)

type ClientConfig struct {
	Host     string
	Port     uint
	Username string
	Password string
}

type ExchangeConfig struct {
	Name      string
	Type      string
	Persisted bool
}

type QueueConfig struct {
	Name      string
	Durable   bool
	Exclusive bool
	Bindings  *[]BindingConfig
}

type BindingConfig struct {
	RoutingKey string
	Exchange   string
}
