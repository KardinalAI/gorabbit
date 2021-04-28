package gorabbit

func SetupMQTT(clientConfig ClientConfig, exchangeConfigs []ExchangeConfig, queueConfigs *[]QueueConfig) error {
	// First we initialize the connection to the AMQP server
	err := initConnection(clientConfig)

	if err != nil {
		return err
	}

	// Defer connection and channel closing until the setup is executed
	defer Connection.Close()
	defer Channel.Close()

	// Loop through all declared exchanges to create them
	for _, conf := range exchangeConfigs {
		err = declareExchange(conf)

		if err != nil {
			return err
		}
	}

	if queueConfigs != nil {
		// Loop through all declared queues to create them
		for _, conf := range *queueConfigs {
			err = declareQueue(conf)

			if err != nil {
				return err
			}

			// Loop through all bindings to the queue to set them up
			if conf.Bindings != nil {
				for _, binding := range *conf.Bindings {
					err = addQueueBinding(conf.Name, binding.RoutingKey, binding.Exchange)

					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func initConnection(config ClientConfig) error {
	_, err := NewMQTTClient(config)

	if err != nil {
		return err
	}

	return nil
}

// declareExchange will initialize you exchange in the RabbitMQ server
func declareExchange(config ExchangeConfig) error {
	err := Channel.ExchangeDeclare(
		config.Name,       // name
		config.Type,       // type
		config.Persisted,  // durable
		!config.Persisted, // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)

	return err
}

// declareQueue will initialize you queue in the RabbitMQ server
func declareQueue(config QueueConfig) error {
	_, err := Channel.QueueDeclare(
		config.Name,      // name
		config.Durable,   // durable
		false,            // delete when unused
		config.Exclusive, // exclusive
		false,            // no-wait
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

// addQueueBinding will bind a queue to an exchange via a specific routing key
func addQueueBinding(queue string, routingKey string, exchange string) error {
	err := Channel.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)

	return err
}
