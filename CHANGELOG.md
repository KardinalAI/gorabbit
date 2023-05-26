# 3.2.1

Added getters methods for the `MQTTClient` and `MQTTManager`

* `GetHost()` Returns the host used to initialize the client/manager.
* `GetPort()` Returns the port used to initialize the client/manager.
* `GetUsername()` Returns the username used to initialize the client/manager.
* `GetVhost()` Returns the vhost used to initialize the client/manager.
* `IsDisabled()` Returns whether the client is disabled or not.

# 3.2.0

Added support for instantiating a client and a manager from environment variables.

* New `NewClientFromEnv` and `NewManagerFromEnv` constructors,
* New `NewClientOptionsFromEnv` and `NewManagerOptionsFromEnv` methods that will parse environment variables
  into `ClientOptions` and `ManagerOptions` objects,
* Added a new model called `RabbitMQEnvs` to easily parse environment variables.
* Updated documentation in the `README.md` file.

# 3.1.1

Upgrade of Golangci-lint version from 1.45 to 1.52.

# 3.1.0

Added TLS and CloudAMQP support.

* `ClientOptions` now accepts two new fields: `Vhost` and `UseTLS`,
* `ManagerOptions` now accepts two new fields: `Vhost` and `UseTLS`.

# 3.0.0

Official V3 release of Gorabbit with **Breaking Changes**

**WHAT'S NEW**

* Gorabbit now offers a client (`MQTTClient`) and a manager (`MQTTManager`) that each serve a different purpose
* Gorabbit can be disabled from an environment variable `GORABBIT_DISABLED`
* Gorrabit mode can be switched from an environment variable `GORABBIT_MODE`
* The client and the manager can be initialized with default values
* The engine behind the client is brand new and more robust
    * Multi-connection support
    * Multi-channel support
    * Strong `keep alive` mechanism if enabled
* Better logs with Logrus
* Added `golangci-lint` linter for a cleaner codebase
* Every piece of code is now documented

**WHAT'S CHANGED**

* The old `MQTTClient` has been split between `MQTTClient` and `MQTTManager`
    * The `MQTTClient` offers basic client operations:
        * Publishing
        * Consuming
        * Ready check
        * Health check
    * The `MQTTManager` offers management operations for the RabbitMQ server:
        * Exchange, queue and binding creation
        * Exchange and queue deletion
        * Queue purge
        * Message push, pop and count
* `ClientConfig` renamed to `ClientOptions` with much more flexibility and customization
* `SendMessage` method split into 2 new methods:
    * `Publish` Simpler publishing
    * `PublishWithOptions` More complex publishing (priority, delivery mode)
* `SubscribeToMessages` method completely revamped to `RegisterConsumer` and now handles all the following operations
  internally:
    * Message retry
    * Message acknowledgement
    * Message negative acknowledgement
    * Message rejection
* `ReadyCheck` method split into 2 new methods:
    * `IsReady` The client is ready but ongoing operations may or may not be down
    * `IsHealthy` The client is ready and all ongoing operations are up and running
* `SubscriptionHealth` renamed to `consumptionHealth` and no longer exported
* Most constants now have their own custom types for type-strict declarations
* Redelivery header renamed from `x-redelivered-count` to `x-death-count`
* Updated go to 1.18
* Better CI scripts
* Externalized some objects and interfaces in their own file

**WHAT'S REMOVED**

* `ListenStatus` completely removed as it has become unnecessary
* `RetryMessage` method completely removed as it is handled internally
* `ConnectionStatus` removed as it is unused
* Internal `consumed` cache removed as it is no longer necessary
* `RabbitServerConfig` struct removed as it is unused
* `AMQPMessage` custom message wrapper removed as it is no longer used
* `EventsExchange` and `CommandsExchange` constants removed due to being too specific

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