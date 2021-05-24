package main

import (
	"gitlab.kardinal.ai/coretech/gorabbit"
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
