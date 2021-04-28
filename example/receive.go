package main

import (
	"gitlab.kardinal.ai/aelkhou/gorabbit"
	"log"
)

func main() {
	clientConfig := gorabbit.ClientConfig{
		Host:     "localhost",
		Port:     5672,
		Username: "guest",
		Password: "guest",
	}

	client, err := gorabbit.NewMQTTClient(clientConfig)

	if err != nil {
		panic(err.Error())
	}

	defer gorabbit.Connection.Close()
	defer gorabbit.Channel.Close()

	messages, err := client.SubscribeToEvents("payload_queue", nil)

	forever := make(chan bool)

	go func() {
		for d := range messages {
			handleEvent(d.Body)
		}
	}()

	<-forever
}

func handleEvent(event []byte) {
	log.Printf("Received message: %d bytes", len(event))
}
