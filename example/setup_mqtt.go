package main

import (
	"gitlab.kardinal.ai/aelkhou/gorabbit"
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
		Type:      gorabbit.Topic,
		Persisted: true,
	}

	eventExchangeConfig := gorabbit.ExchangeConfig{
		Name:      "events_topic",
		Type:      gorabbit.Topic,
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

	err := gorabbit.SetupMQTT(clientConfig, exchanges, &queues)

	if err != nil {
		panic(err.Error())
	}
}
