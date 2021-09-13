package gorabbit

import (
	"fmt"
	"github.com/streadway/amqp"
	"time"
)

// connect will initialize the AMQP connection, Connection and Channel
// given a configuration
func (client *mqttClient) connect() error {
	dialUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	// In debug mode, log the infos
	if client.debug {
		client.logger.WithField("uri", dialUrl).Info("connecting to MQTT server")
	}

	conn, err := amqp.Dial(dialUrl)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("could not connect to MQTT server")
		}

		return err
	}

	connection = conn

	ch, err := conn.Channel()

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("could not open unique channel")
		}

		return err
	}

	err = ch.Qos(10, 0, false)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("failed to define quality of service")
		}

		return err
	}

	channel = ch

	// In debug mode, log the infos
	if client.debug {
		client.logger.Info("connection to MQTT server successful")
	}

	return nil
}

func (client *mqttClient) isOperational() bool {
	return connection != nil && channel != nil && !connection.IsClosed() && !channelClosed
}

// redeliver will resend an MQTT delivery from an acknowledged event
// the event will be sent to the same queue via the same exchanger with
// the same routing key. The event will also hold the same body and properties
func (client *mqttClient) redeliver(event *AMQPMessage) error {
	// Before sending a message, we need to make sure that Connection and Channel are valid
	if !client.isOperational() {
		// In debug mode, log the warning
		if client.debug {
			client.logger.Warn("connection or channel is nil, attempting to reconnect")
		}

		// Otherwise we need to connect again
		err := client.connect()

		if err != nil {
			// In debug mode, log the error
			if client.debug {
				client.logger.WithError(err).Error("could not reconnect to rabbitMQ")
			}

			return err
		}
	}

	// Publish the message via the official amqp package
	// with our given configuration
	err := channel.Publish(
		event.Exchange,   // exchange
		event.RoutingKey, // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: event.ContentType,
			Body:        event.Body,
			Type:        event.RoutingKey,
			Priority:    event.Priority,
			MessageId:   event.MessageId,
			Headers:     event.Headers,
		},
	)

	// In debug mode, log the error
	if err != nil && client.debug {
		client.logger.WithError(err).Error("could not redeliver message")
	}

	return err
}

func (client *mqttClient) cacheConsumedMessage(tag uint64) {
	now := time.Now()
	consumed[tag] = now
}

// declareExchange will initialize you exchange in the RabbitMQ server
func (client *mqttClient) declareExchange(config ExchangeConfig) error {
	err := channel.ExchangeDeclare(
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
func (client *mqttClient) declareQueue(config QueueConfig) error {
	_, err := channel.QueueDeclare(
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
func (client *mqttClient) addQueueBinding(queue string, routingKey string, exchange string) error {
	err := channel.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)

	return err
}

func (client *mqttClient) launchCacheCleanup() {
	time.Sleep(cacheTimeout / 2)

	if client.debug {
		client.logger.WithField("cacheSize", len(consumed)).Info("checking cache")
	}

	if len(consumed) == 0 {
		if client.debug {
			client.logger.Info("cache is empty, skipping cleanup")
		}
		go client.launchCacheCleanup()
		return
	}

	now := time.Now()

	for k, v := range consumed {
		if now.Sub(v) >= cacheTimeout {
			if client.debug {
				client.logger.WithField("deliveryTag", k).Info("cached message expired, removing from cache")
			}
			delete(consumed, k)
		}
	}

	if cacheSize := len(consumed); cacheSize >= cacheLimit {
		if client.debug {
			client.logger.WithField("cacheSize", cacheSize).Info("cached messages exceeded cache limit, emptying 50% of cached")
		}

		emptyCount := cacheLimit/2 + (cacheSize - cacheLimit)
		emptied := 0

		for k, _ := range consumed {
			delete(consumed, k)
			emptied++

			if emptied >= emptyCount {
				if client.debug {
					client.logger.WithField("cacheSize", cacheSize).Info("emptied 50% of cached messages")
				}
				break
			}
		}
	}

	go client.launchCacheCleanup()
}
