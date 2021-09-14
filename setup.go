package gorabbit

import (
	"gopkg.in/yaml.v2"
	"os"
)

func SetupMQTT(clientConfig ClientConfig, serverConfig RabbitServerConfig) error {
	// First we initialize the connection to the AMQP server
	client, err := NewClient(clientConfig)

	if err != nil {
		return err
	}

	// Defer connection and channel closing until the setup is executed
	defer client.Disconnect()

	// Loop through all declared exchanges to create them
	for _, exchange := range serverConfig.Exchanges {
		err = client.CreateExchange(exchange)

		if err != nil {
			return err
		}
	}

	// Loop through all declared queues to create them
	for _, queue := range serverConfig.Queues {
		err = client.CreateQueue(queue)

		if err != nil {
			return err
		}

		// Loop through all bindings to the queue to set them up
		if queue.Bindings != nil {
			for _, binding := range *queue.Bindings {
				err = client.BindExchangeToQueueViaRoutingKey(binding.Exchange, queue.Name, binding.RoutingKey)

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

	return SetupMQTT(clientConfig, *config)
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
