package main

import (
	"gitlab.kardinal.ai/coretech/gorabbit"
	"log"
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

	messages, err := client.SubscribeToEvents("payload_queue", "", false)

	forever := make(chan bool)

	go func() {
		for d := range messages {
			handleEvent(d.Body)
			err = d.Ack(false)

			if err != nil {
				log.Fatal("could not acknowledge delivery")
			}
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")

	<-forever
}

func handleEvent(event []byte) {
	log.Printf("Received message: %s", event)
}
