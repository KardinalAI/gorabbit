package gorabbit

type ClientListeners struct {
	// OnConnectionFailed is called when ConnFailed status is received (The client could not connect to the RabbitMQ server).
	OnConnectionFailed func()

	// OnConnectionLost is called when ConnDown status is received (The client lost connection to the RabbitMQ server).
	OnConnectionLost func()

	// OnChannelDown is called when ChanDown status is received (The client lost connection to the channel).
	OnChannelDown func()

	// OnConnectionUp is called when ConnUp status is received (The client could successfully connect or re-connect to the RabbitMQ server).
	OnConnectionUp func()

	// OnChannelUp is called when ChanUp status is received (The client could successfully connect or re-connect to the channel).
	OnChannelUp func()
}

// DefaultListeners will return a ClientListeners with default ConnectionStatus listeners.
// Each listener will just print the status message via stdLogger.
func DefaultListeners() *ClientListeners {
	logger := &stdLogger{}

	return &ClientListeners{
		OnConnectionFailed: func() {
			logger.Printf("Connection to RabbitMQ server failed")
		},
		OnConnectionLost: func() {
			logger.Printf("Connection to RabbitMQ server lost")
		},
		OnChannelDown: func() {
			logger.Printf("Client channel is down")
		},
		OnConnectionUp: func() {
			logger.Printf("Connection to RabbitMQ server succeeded")
		},
		OnChannelUp: func() {
			logger.Printf("Client channel is up")
		},
	}
}
