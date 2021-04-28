#!/bin/bash
xterm -title "Setup" -e "go run setup_mqtt.go" &&
xterm -title "Sender 1" -e "go run send.go 100 50" &
xterm -title "Sender 2" -e "go run send.go 100 50" &
xterm -title "Sender 3" -e "go run send.go 100 50" &
xterm -title "Receiver 1" -e 'go run receive.go' &
xterm -title "Receiver 2" -e 'go run receive.go' &
xterm -title "Receiver 3" -e 'go run receive.go' &