package channel

import "errors"

var (
	errConnectionClosed = errors.New("connection is closed")
	errConsumerExists   = errors.New("channel already has a consumer")
)
