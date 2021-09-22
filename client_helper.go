package gorabbit

import (
	"context"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

// connect will initialize the AMQP connection, Connection and Channel
// given a configuration
func (client *mqttClient) connect() error {
	client.ctx, client.cancel = context.WithCancel(context.Background())

	dialUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", client.Username, client.Password, client.Host, client.Port)

	// In debug mode, log the infos
	if client.debug {
		client.logger.WithField("uri", dialUrl).Info("connecting to MQTT server")
	}

	var err error
	client.connection, err = amqp.Dial(dialUrl)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("could not connect to MQTT server")
		}

		client.cancel()

		return err
	}

	go client.keepConnectionAlive(dialUrl)

	client.channel, err = client.connection.Channel()

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("could not open unique channel")
		}

		return err
	}

	err = client.channel.Qos(10, 0, false)

	if err != nil {
		// In debug mode, log the error
		if client.debug {
			client.logger.WithError(err).Info("failed to define quality of service")
		}

		return err
	}

	go client.keepChannelAlive()

	// In debug mode, log the infos
	if client.debug {
		client.logger.Info("connection to MQTT server successful")
	}

	return nil
}

func (client *mqttClient) keepConnectionAlive(dialUrl string) {
	for {
		select {
		case <-client.ctx.Done():
			return
		case reason, ok := <-client.connection.NotifyClose(make(chan *amqp.Error)):
			client.status <- ConnDown

			if !ok {
				return
			}

			if client.debug {
				client.logger.WithField("notification", reason).Error("connection closed")
			}

		Loop:
			for {
				// wait reconnectDelay before trying to reconnect
				time.Sleep(reconnectDelay)

				select {
				case <-client.ctx.Done():
					return
				default:
					var err error
					client.connection, err = amqp.Dial(dialUrl)
					if err == nil {
						client.status <- ConnUp
						break Loop
					}
				}
			}
		}
	}
}

func (client *mqttClient) keepChannelAlive() {
	for {
		select {
		case <-client.ctx.Done():
			return
		case reason, ok := <-client.channel.NotifyClose(make(chan *amqp.Error)):
			client.status <- ChanDown

			if !ok {
				return
			}

			if client.debug {
				client.logger.WithField("notification", reason).Error("channel closed")
			}

		Loop:
			for {
				// wait reconnectDelay before trying to reconnect
				time.Sleep(reconnectDelay)

				select {
				case <-client.ctx.Done():
					return
				default:
					var err error
					client.channel, err = client.connection.Channel()
					if err == nil {
						client.status <- ChanUp
						break Loop
					}
				}
			}
		}
	}
}

func (client *mqttClient) isOperational() bool {
	return client.connection != nil && client.channel != nil && !client.connection.IsClosed()
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
	err := client.channel.Publish(
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

// declareExchange will initialize you exchange in the RabbitMQ server
func (client *mqttClient) declareExchange(config ExchangeConfig) error {
	err := client.channel.ExchangeDeclare(
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
	_, err := client.channel.QueueDeclare(
		config.Name,      // name
		config.Durable,   // durable
		false,            // delete when unused
		config.Exclusive, // exclusive
		false,            // no-wait
		nil,
	)

	return err
}

// addQueueBinding will bind a queue to an exchange via a specific routing key
func (client *mqttClient) addQueueBinding(queue string, routingKey string, exchange string) error {
	err := client.channel.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	)

	return err
}
