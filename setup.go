package gorabbit

import (
	"gopkg.in/yaml.v2"
	"os"
)

func SetupMQTT(clientConfig ClientConfig, serverConfig RabbitServerConfig) error {
	// First we initialize the connection to the AMQP server
	err := initConnection(clientConfig)

	if err != nil {
		return err
	}

	// Defer connection and channel closing until the setup is executed
	defer Connection.Close()
	defer Channel.Close()

	// Loop through all declared exchanges to create them
	for _, exchange := range serverConfig.Exchanges {
		err = declareExchange(exchange)

		if err != nil {
			return err
		}
	}

	// Loop through all declared queues to create them
	for _, queue := range serverConfig.Queues {
		err = declareQueue(queue)

		if err != nil {
			return err
		}

		// Loop through all bindings to the queue to set them up
		if queue.Bindings != nil {
			for _, binding := range *queue.Bindings {
				err = addQueueBinding(queue.Name, binding.RoutingKey, binding.Exchange)

				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func SetupMQTTFromYML(clientConfig ClientConfig, filePath string) error {
	// First we want to retrieve the config from the YML file and Decode it to its struct
	config, err := loadYmlFileFromPath(filePath)

	if err != nil {
		return err
	}

	// Then we initialize the connection to the AMQP server
	err = initConnection(clientConfig)

	if err != nil {
		return err
	}

	// Defer connection and channel closing until the setup is executed
	defer Connection.Close()
	defer Channel.Close()

	// Loop through all declared exchanges to create them
	for _, exchange := range config.Exchanges {
		err = declareExchange(exchange)

		if err != nil {
			return err
		}
	}

	// Loop through all declared queues to create them
	for _, queue := range config.Queues {
		err = declareQueue(queue)

		if err != nil {
			return err
		}

		// Loop through all bindings to the queue to set them up
		if queue.Bindings != nil {
			for _, binding := range *queue.Bindings {
				err = addQueueBinding(queue.Name, binding.RoutingKey, binding.Exchange)

				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func loadYmlFileFromPath(path string) (*RabbitServerConfig, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	file, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	config := RabbitServerConfig{}
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)

	if err != nil {
		return nil, err
	}

	return &config, nil
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
