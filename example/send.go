package main

import (
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

	defer gorabbit.Connection.Close()
	defer gorabbit.Channel.Close()

	err = client.SendEvent("payloads_topic","event.payload.heavy", []byte("Hello World! This is a dummy heavy payload :)"))

	if err != nil {
		panic(err.Error())
	}
}