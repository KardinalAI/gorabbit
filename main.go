package gorabbit

import (
	"github.com/sirupsen/logrus"
	"time"
)

func NewClient(config ClientConfig) (MQTTClient, error) {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		debug:    false,
	}

	client.status = make(chan ConnectionStatus)

	err := client.connect()

	if err != nil {
		// if maxRetry is set and greater than 0
		// we recursively call the constructor to
		// retry connecting
		if config.MaxRetry > 0 {
			config.MaxRetry -= 1
			time.Sleep(config.RetryDelay)

			return NewClient(config)
		}

		return nil, err
	}

	if consumed == nil {
		consumed = NewTTLMap(cacheLimit, cacheTTL)
	}

	return client, nil
}

func NewClientDebug(config ClientConfig, logger *logrus.Logger) (MQTTClient, error) {
	client := &mqttClient{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		debug:    true,
		logger:   logger,
	}

	client.status = make(chan ConnectionStatus)

	err := client.connect()

	if err != nil {
		// if maxRetry is set and greater than 0
		// we recursively call the constructor to
		// retry connecting
		if config.MaxRetry > 0 {
			config.MaxRetry -= 1
			time.Sleep(config.RetryDelay)

			// In debug mode, log the info
			if client.debug {
				client.logger.Info("retrying to connect to MQTT server")
			}

			return NewClient(config)
		}

		return nil, err
	}

	if consumed == nil {
		consumed = NewTTLMap(cacheLimit, cacheTTL)
	}

	return client, nil
}
