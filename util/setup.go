package util

func SetupMQTT(clientConfig ClientConfig, exchangeConfigs []ExchangeConfig, queueConfigs *[]QueueConfig) error {
	err := initConnection(clientConfig)

	if err != nil {
		return err
	}

	defer Connection.Close()
	defer Channel.Close()

	for _, conf := range exchangeConfigs {
		err = declareExchange(conf)

		if err != nil {
			return err
		}
	}

	if queueConfigs != nil {
		for _, conf := range *queueConfigs {
			err = declareQueue(conf)

			if err != nil {
				return err
			}

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
