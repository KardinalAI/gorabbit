package main

import (
	"fmt"
	"gitlab.kardinal.ai/aelkhou/gorabbit"
	"io/ioutil"
	"os"
	"strconv"
)

var client gorabbit.MQTTClient

func main() {
	argsList := os.Args
	loadMultiplierString := ""
	operationsMultiplierString := ""

	if len(argsList) == 1 {
		loadMultiplierString = "17"
		operationsMultiplierString = "2"
	} else if len(argsList) == 2 {
		loadMultiplierString = argsList[1]
		operationsMultiplierString = "2"
	} else {
		loadMultiplierString = argsList[1]
		operationsMultiplierString = argsList[2]
	}

	fmt.Println(fmt.Sprintf("Simulation multiplier: %s%% load", loadMultiplierString))
	fmt.Println(fmt.Sprintf("Operations multiplier: %s repetitions per day", operationsMultiplierString))

	loadMultiplier, err := strconv.ParseUint(loadMultiplierString, 10, 64)

	if err != nil {
		panic("Invalid argument")
	}

	operationsMultiplier, err := strconv.ParseUint(operationsMultiplierString, 10, 64)

	if err != nil {
		panic("Invalid argument")
	}

	heavyPayloadMultiplier := int(1000 * (float64(loadMultiplier) / 100.0))
	mediumPayloadMultiplier := int(5000 * (float64(loadMultiplier) / 100.0))
	smallPayloadMultiplier := int(15000 * (float64(loadMultiplier) / 100.0))

	fmt.Println()
	fmt.Println(fmt.Sprintf("Heavy load multiplier: %d events, for a total of %d events in one day", heavyPayloadMultiplier, heavyPayloadMultiplier*int(operationsMultiplier)))
	fmt.Println(fmt.Sprintf("Medium load multiplier: %d events, for a total of %d events in one day", mediumPayloadMultiplier, mediumPayloadMultiplier*int(operationsMultiplier)))
	fmt.Println(fmt.Sprintf("Small load multiplier: %d events, for a total of %d events in one day", smallPayloadMultiplier, smallPayloadMultiplier*int(operationsMultiplier)))

	initAMQPConnection()

	launchSimulation([]int{heavyPayloadMultiplier, mediumPayloadMultiplier, smallPayloadMultiplier}, int(operationsMultiplier))
}

func launchSimulation(loads []int, repetitions int) {
	heavyPayload := readPayload("test-big.json")
	mediumPayload := readPayload("test-medium.json")
	smallPayload := readPayload("test-small.json")

	if heavyPayload == nil || mediumPayload == nil || smallPayload == nil {
		panic("Could not load and read payloads")
	}

	defer gorabbit.Connection.Close()
	defer gorabbit.Channel.Close()

	forever := make(chan bool)
	for i := 0; i < repetitions; i++ {
		go sendMessages("event.payload.heavy", heavyPayload, loads[0])
		go sendMessages("event.payload.medium", mediumPayload, loads[1])
		go sendMessages("event.payload.small", smallPayload, loads[2])
	}
	<-forever
}

func readPayload(file string) []byte {
	jsonFile, err := os.Open(file)

	if err != nil {
		fmt.Println("Could not open test-big.json file")
		return nil
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	return byteValue
}

func initAMQPConnection() {
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
}

func sendMessages(routingKey string, payload []byte, amount int) {
	for i := 0; i < amount; i++ {
		err := client.SendEvent(
			"payloads_topic", // exchange
			routingKey,       // routing key
			payload,
		)

		if err != nil {
			fmt.Println(err.Error())
		}
	}
}
