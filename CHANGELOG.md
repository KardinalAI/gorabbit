# 1.1.0
Added support for dynamic queue, exchange et binding management
* MQTT Client new methods:
    * `CreateQueue`: Creates a queue from given configuration
    * `CreateExchange`: Creates exchange from given configuration
    * `BindExchangeToQueueViaRoutingKey`: Binds exchange to queue via given routing key
    * `QueueIsEmpty`: Returns whether a queue is empty or not. Returns an error if the queue doesn't exist
    * `GetNumberOfMessages`: Returns the number of messages contained in a queue. Returns an error if the queue doesn't exist
    * `PopMessageFromQueue`: Pop message from queue (Retrieve first out). Returns an error if the queue is empty
    * `PurgeQueue`: Purges all messages from a queue. Returns an error if the queue doesn't exist
    * `DeleteQueue`: Deletes a queue. Returns an error if the queue doesn't exist
    * `QueueIsEmpty`: Deletes an exchange. Returns an error if the exchange doesn't exist
* MQTT Setup simplifications:
    * Use MQTT Client for setup operations
    * Moved helper functions `declareExchange`, `declareQueue` and `addQueueBinding` to MQTT Client as private methods.


# 1.0.0
Official stable release of Gorabbit
* Removed logging module
* Modified `NewClientDebug` constructor to take a `logrus.Logger` logger as a parameter
* Working and tested retry strategy for failed messages

# 0.1.5
(**NOTE**: This version has **Breaking Changes** and should have been released in v1.0.0)

Added redelivery and max retry feature
* Added `RetryMessage` functionality that acknowledges an event then re-sends it to the same queue
* Redelivered messages are acknowledged and then re-sent with incremented `x-redelivered-count` header
* The redelivery strategy is based on the `x-redelivered-count` header of an event
* Removed `connect` method from the client interface
* Renamed `SendEvent` method to `SendMessage`
* Renamed `SubscribeToEvents` method to `SubscribeToMessages`
* Renamed `NewMQTTClient` factory to `NewClient`
* Introduced `NewClientDebug` factory that will enable logs
* Introduced logs in all operations


# 0.1.4
* Renamed `MessageType` to `AMQPMessage`
* `AMQPMessage` inherits `amqp.Delivery`
* Made `Subscribe` method return a channel of `AMQPMessage` instead of amqp.Delivery
* Deleted utils packed
* Renamed `message_type_parser.go` to `amqp_message_parser.go` and moved the file to gorabbit package
* `ClientConfig` added `MaxRetry` and `RetryDelay` properties
* Implemented a retry strategy on the `Connect` method


# 0.1.3
Fixed readme file typo in Installation section

# 0.1.2

* Removed example project
* Added CHANGELOG.md

# 0.1.1

Removed subscription consumer pointer and converted it to a string. Accept empty string consumer to generate
automatically random unique identifier.

# 0.1.0

Initial library version

* MQTTClient
* MQTTSetup (Struct & YML)
* Constants
* Models
* Example project