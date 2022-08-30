# 2.2.0

**WHAT'S NEW**

* Added extra logs for debugging
* Introduced new `MessagePriority` type
* Added new *optional* property `Mode` in `ClientConfig` to control `debug` or `release` modes.
  The mode can also alternatively be passed via an environment variable `GORABBIT_MODE`
* Added new *optional* property `OnConnectionStatusChanged` in `ClientConfig` to control the connection status
  change event synchronously.
* Added `golangci-lint` rules and `gitlab-ci` lint test step

**WHAT'S CHANGED**

* Updated go to 1.18
* Deprecated `NewClientDebug` in favor of `NewClient` with `Mode`
* Deprecated `RabbitServerConfig` struct that is no longer used

**WHAT'S FIXED**

* Re-connection sometimes not working due to the buffered channel of `ConnectionStatus` reaching a deadlock.
* Fixed some typos in comments

# 2.1.0

* Introduced Health Check for failed subscriptions
* Client `ReadyCheck` method returns readiness and health check
* New `SubscriptionsHealth` model

# 2.0.0

Official V2 release of Gorabbit

**WHAT'S NEW**

* Introduced a connection manager to handle `amqp.Connection` and `amqp.Channel` management
* Better reconnection and keep alive logic on channel or connection closed
* New `ReadyCheck` method that check if everything is up and running
* New Logging interface

**WHAT'S CHANGED**

* Everything is now handled by `connectionManager`
* `connectionManager` overwrites amqp channel methods to check for connection status first
* Removed `logrus` dependency for logging
* Added new Connection Status `ConnFailed` to handle failed initial connection
* Reduced project files

**BREAKING CHANGES**

* `ClientConfig` now takes a `KeepAlive` boolean attribute instead of `MaxRetry` and `RetryDelay`
* `NewClient` and `NewClientConfig` no longer return an error
* `NewClientDebug` no longer takes a `logger` parameter
* `QueueIsEmpty` has been removed as `GetNumberOfMessages` is enough for that evaluation
* Removed `SetupMQTT` and `SetupMQTTFromYML` methods and logic as their functionalities are fully covered by the client

# 1.4.2

Small bug fixes

* Modified `Ack`, `Nack` and `Reject` methods of `AMQPMessage` to return no error if already consumed
* Modified `PopMessageFromQueue` method to cache auth acknowledged message

# 1.4.1

Migration from https://github.com/streadway/amqp to https://github.com/rabbitmq/amqp091-go

* Replaced `"github.com/streadway/amqp"` import to `amqp "github.com/rabbitmq/amqp091-go"` in all files
* Fixed `Disconnect` error caused by channel being already closed
* Introduced context management to stop the auto-reconnect process on client manual disconnect

# 1.4.0

* Reverted changes in the `RetryMessage` method signature in v1.3.0
* Reverted changes done for handling connection close in v1.2.0

Solidified connection and channel management, as well as message ack, nack and reject management:

* Added a custom TTL cache for consumed messages to deal with ack, nack and reject of already consumed messages
* Added new `ConnnectionStatus` type
* Added new `ListenStatus` method that returns a stream of `ConnectionStatus`
* Overwrote `Ack`, `Nack` and `Reject` methods on `AMQPMessage` to handle errors internally
* Split client constructors, moved from `client.go` to `main.go`
* Split client helpers, moved from `client.go` to `client_helper.go`
* Created `TTLMap` custom type to handle a map with a TTL for each object
* Enriched sent messages with a timestamp

# 1.3.0

**BREAKING CHANGES**

* Changed `RetryMessage` method signature to take a `shouldAck` flag
* Added condition on message ack based on `shouldAck` value

# 1.2.0

**BREAKING CHANGES**

Added connection close listener

* New `NotifyClose` method to listen to connection close
* Updated `SubscribeToMessages` method signature to take a context as argument and deal with it being done

# 1.1.0

Added support for dynamic queue, exchange et binding management

* MQTT Client new methods:
    * `CreateQueue`: Creates a queue from given configuration
    * `CreateExchange`: Creates exchange from given configuration
    * `BindExchangeToQueueViaRoutingKey`: Binds exchange to queue via given routing key
    * `QueueIsEmpty`: Returns whether a queue is empty or not. Returns an error if the queue doesn't exist
    * `GetNumberOfMessages`: Returns the number of messages contained in a queue. Returns an error if the queue doesn't
      exist
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