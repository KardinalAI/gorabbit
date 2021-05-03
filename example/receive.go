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

	messages, err := client.SubscribeToEvents("payload_queue", nil, false)

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
