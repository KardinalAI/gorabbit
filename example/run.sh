#!/bin/bash
xterm -title "Setup" -e "go run setup_from_yml.go" &&
  xterm -title "Receiver" -e 'go run receive.go' &
xterm -title "Sender" -e "go run send.go" &
