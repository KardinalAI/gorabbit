package main

import (
	"fmt"
	"gitlab.kardinal.ai/aelkhou/gorabbit"
)

var client gorabbit.MQTTClient

func main() {
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

	defer client.Disconnect()

	for i := 0; i < 100000; i++ {
		err = client.SendEvent("payloads_topic", "event.payload.heavy", gorabbit.PriorityHigh, []byte(fmt.Sprintf("I am sending message number: %d", i+1)))

		if err != nil {
			panic(err.Error())
		}
	}
}
