package gorabbit

import "errors"

const (
	RedeliveryHeader = "x-redelivered-count"
	MaxRetryHeader   = "x-death"
)

// Connection Types.

type ConnectionType string

const (
	Consumer  ConnectionType = "consumerConnection"
	Publisher ConnectionType = "publisherConnection"
)

// Priority Levels.

type MessagePriority uint8

const (
	PriorityLowest  MessagePriority = 1
	PriorityVeryLow MessagePriority = 2
	PriorityLow     MessagePriority = 3
	PriorityMedium  MessagePriority = 4
	PriorityHigh    MessagePriority = 5
	PriorityHighest MessagePriority = 6
)

func (m MessagePriority) Uint8() uint8 {
	return uint8(m)
}

// Delivery Modes.

type DeliveryMode uint8

const (
	Transient  DeliveryMode = 1
	Persistent DeliveryMode = 2
)

func (d DeliveryMode) Uint8() uint8 {
	return uint8(d)
}

// Logging Modes.
const (
	Release = "release"
	Debug   = "debug"
)

func isValidMode(mode string) bool {
	return mode == Release || mode == Debug
}

// Errors.
var (
	errEmptyURI                = errors.New("amqp uri is empty")
	errDeliveryNotInitialized  = errors.New("delivery not initialized")
	errConnectionClosed        = errors.New("connection is closed")
	errChannelClosed           = errors.New("channel is closed")
	errConsumerAlreadyExists   = errors.New("consumerConnection already exists")
	errConsumerNotInitialized  = errors.New("consumerConnection is not initialized")
	errPublisherNotInitialized = errors.New("publisherConnection is not initialized")
)
