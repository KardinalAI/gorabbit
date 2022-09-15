# Gorabbit

Gorabbit is a wrapper that provides high level and robust RabbitMQ operations through a client or a manager.

This wrapper depends on the official [Go RabbitMQ plugin](https://github.com/rabbitmq/amqp091-go).

## Installation

### Go module

```bash
go get gitlab.kardinal.ai/coretech/gorabbit/v3
```

### Possible issues

If you get an error regarding the host not found, run the following command:

```bash
git config --global --add url.git@gitlab.kardinal.ai:.insteadOf https://gitlab.kardinal.ai/
```

If you are having trouble updating the package to the latest version, run the following commands:

```bash
go env -w GONOPROXY="gitlab.kardinal.ai/*"
go env -w GONOSUMDB="gitlab.kardinal.ai/*"
```

## Client

The gorabbit client offers 2 main functionalities:

* Publishing
* Consuming

Additionally, the client also provides a ready check and a health check.

### Initialization

A client can be initialized via the constructor `NewClient`. This constructor takes `ClientOptions` as an optional
parameter.

### Options

| Property            | Description                                             | Default Value |
|---------------------|---------------------------------------------------------|---------------|
| Host                | The hostname of the RabbitMQ server                     | 127.0.0.1     |
| Port                | The port of the RabbitMQ server                         | 5672          |
| Username            | The plain authentication username                       | guest         |
| Password            | The plain authentication password                       | guest         |
| KeepAlive           | The flag that activates retry and re-connect mechanisms | true          |
| RetryDelay          | The delay between each retry and re-connection          | 3 seconds     |
| MaxRetry            | The max number of message retry if it failed to process | 5             |
| PublishingCacheTTL  | The time to live for a failed publish when set in cache | 60 seconds    |
| PublishingCacheSize | The max number of failed publish to add into cache      | 128           |
| Mode                | The mode defines whether logs are shown or not          | Release       |

### Client with default options

Passing `nil` options will trigger the client to use default values (host, port, credentials, etc...)
via `DefaultClientOptions()`.

```go
client := gorabbit.NewClient(nil)
```

You can also explicitly pass `DefaultClientOptions()` for a cleaner initialization.

```go
client := gorabbit.NewClient(gorabbit.DefaultClientOptions())
```

Finally, passing a `NewClientOptions()` method also initializes default values if not overwritten.

```go
client := gorabbit.NewClient(gorabbit.NewClientOptions())
```

### Client with custom options

We can input custom values for a specific property, either via the built-in builder or via direct struct initialization.

#### Using the builder

`NewClientOptions()` and `DefaultClientOptions()` both return an instance of `*ClientOptions` that can act as a builder.

```go
options := gorabbit.NewClientOptions()
    .SetMode(gorabbit.Debug)
    .SetCredentials("root", "password")
    .SetRetryDelay(5 * time.Second)

client := gorabbit.NewClient(options)
```

> :information_source: There is a setter method for each property.

#### Using struct initialization

`ClientOptions` is an exported type so it can be used directly.

```go
options := gorabbit.ClentOptions {
    Host:     "localhost",
    Port:     5673,
    Username: "root",
    Password: "password"
    ...
}

client := gorabbit.NewClient(&options)
```

> :warning: Direct initialization via the struct **does not use default values on missing properties**, so be sure to
> fill
> in every property available.

### Environment Variables

The client's `Mode` can also be set via an environment variable that will **override** the manually entered value.

```dotenv
GORABBIT_MODE: debug    # possible values: release or debug
```

The client can also be completely disabled via the following environment variable:

```dotenv
GORABBIT_DISABLED: true     # possible values: true, false, 1, or 0 
```

### Disconnection

When a client is initialized, to prevent a leak, always disconnect it when no longer needed.

```go
client := gorabbit.NewClient(gorabbit.DefaultClientOptions())
defer client.Disconnect()
```

### Publishing

To send a message, the client offers two simple methods: `Publish` and `PublishWithOptions`. The required arguments for
publishing are:

* Exchange (which exchange the message should be sent to)
* Routing Key
* Payload (`interface{}`, the object will be unmarshalled internally)

Example of sending a simple string

```go
err := client.Publish("events_exchange", "event.foo.bar.created", "foo string")
```

Example of sending an object

```go
type foo struct {
    Action string
}

err := client.Publish("events_exchange", "event.foo.bar.created", foo{Action: "bar"})
```

Optionally, you can set the message's `Priorty` and `DeliveryMode` via the `PublishWithOptions` method.

```go
options := gorabbit.SendOptions()
    .SetPriority(gorabbit.PriorityMedium)
    .SetDeliveryMode(gorabbit.Persistent)

err := client.PublishWithOptions("events_exchange", "event.foo.bar.created", "foo string", options)
```

## Launch Local RabbitMQ Server

To run a local rabbitMQ server quickly with a docker container, simply run the following command:

```bash
docker run -it --rm --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:3-management
```

It will launch a local RabbitMQ server mapped on port 5672, and the management dashboard will be mapped on
port 15672 accessible on localhost:15672 with a username "guest" and password "guest"
