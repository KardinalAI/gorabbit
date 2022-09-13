package gorabbit

type MQTTManager interface {
	// CreateQueue will create a new queue from QueueConfig.
	CreateQueue(config QueueConfig) error

	// CreateExchange will create a new exchange from ExchangeConfig.
	CreateExchange(config ExchangeConfig) error

	// BindExchangeToQueueViaRoutingKey will bind an exchange to a queue via a given routingKey.
	// Returns an error if the connection to the RabbitMQ server is down or if the exchange or queue does not exist.
	BindExchangeToQueueViaRoutingKey(exchange, queue, routingKey string) error

	// GetNumberOfMessages retrieves the number of messages currently sitting in a given queue.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	GetNumberOfMessages(config QueueConfig) (int, error)

	// PopMessageFromQueue retrieves the first message of a queue. The message can then be auto-acknowledged or not.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist or is empty.
	PopMessageFromQueue(queue string, autoAck bool) (*amqpDelivery, error)

	// PurgeQueue will empty a queue of all its current messages.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	PurgeQueue(queue string) error

	// DeleteQueue permanently deletes an existing queue.
	// Returns an error if the connection to the RabbitMQ server is down or the queue does not exist.
	DeleteQueue(queue string) error

	// DeleteExchange permanently deletes an existing exchange.
	// Returns an error if the connection to the RabbitMQ server is down or the exchange does not exist.
	DeleteExchange(exchange string) error
}
