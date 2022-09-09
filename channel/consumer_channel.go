package channel

import (
	"context"
)

type consumerChannel struct {
	amqpChannel
	consumer          *MessageConsumer
	consumptionCtx    context.Context
	consumptionCancel context.CancelFunc
	maxRetry          uint
}

func (c *consumerChannel) registerConsumer(consumer MessageConsumer) error {
	if c.consumer != nil {
		return errConsumerExists
	}

	c.consumer = &consumer

	if c.Ready() {
		go c.consume()
	}

	return nil
}

func (c *consumerChannel) onChannelOpened() {
	c.consumptionCtx, c.consumptionCancel = context.WithCancel(c.ctx)

	go c.consume()
}

func (c *consumerChannel) onChannelClosed() {
	c.consumptionCancel()
}

func (c *consumerChannel) consume() {
	if c.consumer == nil {
		return
	}

	messages, err := c.channel.Consume(c.consumer.Queue, c.consumer.Name, c.consumer.AutoAck, false, false, false, nil)
	if err != nil {
		return
	}

	for {
		select {
		case <-c.consumptionCtx.Done():
			return
		case message := <-messages:
			// When a queue is deleted midway, a message with no delivery tag or ID is received.
			if message.DeliveryTag == 0 && message.MessageId == "" {
				return
			}

			if handler, found := c.consumer.Handlers[message.RoutingKey]; found {
				err = handler(message.Body)

				// If we receive an error and the retry mechanism is activated, we retry the message.
				if err != nil && c.maxRetry > 0 {
					// retryErr := c.retryMessage(message, c.maxRetry)

					// If the retry mechanism failed, we call the OnRetryError hook if it exists.
					// if retryErr != nil {
					//	if c.consumer.OnRetryError != nil {
					//		c.consumer.OnRetryError(message.MessageId, retryErr)
					//	}
					//}
				}
			}
		}
	}
}
