package main

import (
	"gitlab.kardinal.ai/coretech/gorabbit"
)

func main() {
	clientConfig := gorabbit.ClientConfig{
		Host:     "localhost",
		Port:     5672,
		Username: "guest",
		Password: "guest",
	}

	payloadExchangeConfig := gorabbit.ExchangeConfig{
		Name:      "payloads_topic",
		Type:      "topic",
		Persisted: true,
	}

	eventExchangeConfig := gorabbit.ExchangeConfig{
		Name:      "events_topic",
		Type:      "topic",
		Persisted: true,
	}

	payloadBindings := []gorabbit.BindingConfig{
		{
			RoutingKey: "*.payload.#",
			Exchange:   payloadExchangeConfig.Name,
		},
	}

	eventBindings := []gorabbit.BindingConfig{
		{
			RoutingKey: "*.event.#",
			Exchange:   eventExchangeConfig.Name,
		},
	}

	payloadQueueConfig := gorabbit.QueueConfig{
		Name:      "payload_queue",
		Durable:   true,
		Exclusive: false,
		Bindings:  &payloadBindings,
	}

	eventQueueConfig := gorabbit.QueueConfig{
		Name:      "event_queue",
		Durable:   true,
		Exclusive: false,
		Bindings:  &eventBindings,
	}

	exchanges := []gorabbit.ExchangeConfig{payloadExchangeConfig, eventExchangeConfig}
	queues := []gorabbit.QueueConfig{payloadQueueConfig, eventQueueConfig}

	serverConfig := gorabbit.RabbitServerConfig{
		Exchanges: exchanges,
		Queues:    queues,
	}

	err := gorabbit.SetupMQTT(clientConfig, serverConfig)

	if err != nil {
		panic(err.Error())
	}
}
