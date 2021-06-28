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