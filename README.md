# Go Rabbit

Go Rabbit is a go module that offers an easy RabbitMQ client to be used in our microservices.

This module depends on the official [Go RabbitMQ library](https://github.com/streadway/amqp)

## Installation

To add this module to your project, simple run the following command:

```bash
go get gitlab.kardinal.ai/aelkhou/gorabbit
```

If you get an error regarding the host not found, run the following command:

```bash
git config --global --add url.git@git config --global --add url.git@gitlab.kardinal.ai:.insteadOf https://gitlab.kardinal.ai/gitlab.kardinal.ai:.insteadOf https://gitlab.kardinal.ai/
```

Then if you are having trouble updating the package to the latest version, run the following commands:

```bash
go env -w GONOPROXY="gitlab.kardinal.ai/*"
go env -w GONOSUMDB="gitlab.kardinal.ai/*"
```

## Usage

This module contains 3 main functionalities:

* Queues, Exchangers and Bindings Configurator from Struct
* Queues, Exchangers and Bindings Configurator from YML
* Event Sender
* Event Subscriber

### Queues, Exchangers and Bindings Configurator from Struct

You can declare a new queue:

```go
queue := gorabbit.QueueConfig{
    Name:      "my_queue",
    Durable:   true,
    Exclusive: false,
    Bindings:  nil,
}
```

* Name: The unique queue identifier
* Durable: Determines whether a queue is persisted or deleted as soon as the service is stopped or killed
* Exclusive: An exclusive queue is only accessible by the connection that declared it
* Bindings: The list of bindings to be applied to that queue

You can declare a new binding:

```go
binding := gorabbit.BindingConfig{
    RoutingKey: "routing.key",
    Exchange:   "exchange_name",
}
```

* RoutingKey: The "route" you want to bind to the exchanger
* Exchange: The name of the exchanger to be bound to the queue

You can declare a new exchange:

```go
exchange := gorabbit.ExchangeConfig{
    Name:      "exchange_name",
    Type:      "topic",
    Persisted: true,
}
```

* Name: The unique identifier for the exchange
* Type: The exchange type (For now we only support "topic")
* Persisted: Determines whether the exchange is persisted on server stop/kill/restart

Finally, to run the setup after declaring all the configs, you need to connect to the RabbitMQ server and execute the
setup. This can be simplified by simply calling the SetupMQTT function that the package provides:

```go
clientConfig := gorabbit.ClientConfig{
    Host:     "localhost",
    Port:     5672,
    Username: "guest",
    Password: "guest",
}

...

serverConfig := gorabbit.RabbitServerConfig{
    Exchanges: exchanges,
    Queues: queues,
}

err := gorabbit.SetupMQTT(clientConfig, serverConfig)

if err != nil {
    panic(err.Error())
}

```

#### Complete Example

```go
package main

import (
	"gitlab.kardinal.ai/aelkhou/gorabbit"
)

func main() {
	clientConfig := gorabbit.ClientConfig{
		Host:     "localhost",
		Port:     5672,
		Username: "guest",
		Password: "guest",
	}

	payloadExchangeConfig := gorabbit.ExchangeConfig{
		Name:      "payloads_topic",
		Type:      "topic",
		Persisted: true,
	}

	eventExchangeConfig := gorabbit.ExchangeConfig{
		Name:      "events_topic",
		Type:      "topic",
		Persisted: true,
	}

	payloadBindings := []gorabbit.BindingConfig{
		{
			RoutingKey: "*.payload.#",
			Exchange:   payloadExchangeConfig.Name,
		},
	}

	eventBindings := []gorabbit.BindingConfig{
		{
			RoutingKey: "*.event.#",
			Exchange:   eventExchangeConfig.Name,
		},
	}

	payloadQueueConfig := gorabbit.QueueConfig{
		Name:      "payload_queue",
		Durable:   true,
		Exclusive: false,
		Bindings:  &payloadBindings,
	}

	eventQueueConfig := gorabbit.QueueConfig{
		Name:      "event_queue",
		Durable:   true,
		Exclusive: false,
		Bindings:  &eventBindings,
	}

	exchanges := []gorabbit.ExchangeConfig{payloadExchangeConfig, eventExchangeConfig}
	queues := []gorabbit.QueueConfig{payloadQueueConfig, eventQueueConfig}

	serverConfig := gorabbit.RabbitServerConfig{
		Exchanges: exchanges,
		Queues: queues,
	}

	err := gorabbit.SetupMQTT(clientConfig, serverConfig)

	if err != nil {
		panic(err.Error())
	}
}
```

### Queues, Exchangers and Bindings Configurator from YML

You can write the full configuration in a YML file and then parse it to set up your
RabbitMQ server with queues, exchanges and bindings.

The following is an example YML configuration file:

```yaml
exchanges:
  - name: "payloads_topic"
    type: "topic"
    persisted: true
  - name: "events_topic"
    type: "topic"
    persisted: true

queues:
  - name: "payload_queue"
    durable: true
    exclusive: false
    bindings:
      - routing_key: "*.payload.#"
        exchange: "payloads_topic"
  - name: "event_queue"
    durable: true
    exclusive: false
    bindings:
      - routing_key: "*.event.#"
        exchange: "events_topic"
```

Then, you simply have to declare your file name/path before passing it to the YML configurator function

```go
fileName := "./configurator.yml"
err := gorabbit.SetupMQTTFromYML(clientConfig, fileName)
```


#### Complete Example

```go
package main

import (
	"gitlab.kardinal.ai/aelkhou/gorabbit"
)

func main() {
	clientConfig := gorabbit.ClientConfig{
		Host:     "localhost",
		Port:     5672,
		Username: "guest",
		Password: "guest",
	}

	fileName := "./configurator.yml"

	err := gorabbit.SetupMQTTFromYML(clientConfig, fileName)

	if err != nil {
		panic(err.Error())
	}
}
```

## Event Sender

To send an event through MQTT, you need to first declare a client that will automatically connect to the RabbitMQ
server.

```go
var client gorabbit.MQTTClient

clientConfig := gorabbit.ClientConfig{
    Host:     "localhost",
    Port:     5672,
    Username: "guest",
    Password: "guest",
}

c, err := gorabbit.NewMQTTClient(clientConfig)

if err != nil {
    panic(err.Error())
}

client = c
```

Once the client initialized, the Connection and Channel objects will be initialized but do not close at any point yet.
It is very important to properly manage the closure of both streams otherwise you might not send anything. At the
correct point in the microservice, consider adding the following two lines of code:

```go
defer gorabbit.Connection.Close()
defer gorabbit.Channel.Close()
```

For example in Gin, those should be declared in your main function before:
```go 
router.Run()
```
if you intend to keep an open connection while the app is running.

Once your client is initialized, to send an event simply call the integrated function as following:
```go
client.SendEvent("exchange", "routingKey", payload)
```
where the payload should be a []byte

## Event Subscriber

The client initialization is the same as the Event Sender.

Once the client is initialized, you can subscribe to an event by using the integrated function as following:
```go
messages, err := client.SubscribeToEvents("queue_name", "consumer_name")
```

where messages is an asynchronous incoming stream of events that needs to be properly consumed preferably in a go routine.

For example, if you are outside a Gin environment, you need to listen to the stream continuously:
```go
forever := make(chan bool)

go func() {
	for d := range messages {
		log.Printf("Received message: %s", d.Body)
	}
}()
	
<-forever
```

whereas in Gin, the program itself is continuous, so creating a continuous loop is unnecessary, so you can get rid of the "forever" channel.


## Launch Local RabbitMQ Server

To run a local rabbitMQ server quickly with a docker container, simply run the following command:
```bash
docker run -it --rm --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:3-management
```

It will launch a local RabbitMQ server mapped on port 5672, and the management dashboard will be mapped on
port 15672 accessible on localhost:15672 with a username "guest" and password "guest"