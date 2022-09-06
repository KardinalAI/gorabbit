package gorabbit

// messageBuilder will act as a builder for sending an MQTT message.
type messageBuilder struct {
	// client holds an MQTTClient for access to the sending operations.
	client MQTTClient

	// exchange is the MQTT exchange the message is meant to reach.
	exchange string

	// routingKey is the MQTT routing key the message is meant to trigger.
	routingKey string

	// payload is the MQTT message payload in []byte.
	payload []byte

	// options will hold sendOptions for the MQTT message.
	options *sendOptions
}

// newMessageBuilder returns an instance of a messageBuilder.
// Here the MQTTClient is passed holding the necessary operations to execute the message.
func newMessageBuilder(client MQTTClient) *messageBuilder {
	return &messageBuilder{client: client}
}

// Exchange simply sets the exchange.
func (m *messageBuilder) Exchange(exchange string) *messageBuilder {
	m.exchange = exchange

	return m
}

// RoutingKey simply sets the routing key.
func (m *messageBuilder) RoutingKey(routingKey string) *messageBuilder {
	m.routingKey = routingKey

	return m
}

// Payload simply sets the payload.
func (m *messageBuilder) Payload(payload []byte) *messageBuilder {
	m.payload = payload

	return m
}

// Options simply sets the options.
func (m *messageBuilder) Options(options *sendOptions) *messageBuilder {
	m.options = options

	return m
}

// Execute will use the MQTTClient to send the message.
func (m *messageBuilder) Execute() error {
	return m.client.SendMessageWithOptions(m.exchange, m.routingKey, m.payload, m.options)
}
