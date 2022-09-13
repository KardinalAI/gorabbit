package gorabbit

import "errors"

type ConnectionStatus string

const (
	ConnClosed ConnectionStatus = "connClosed"
	ConnFailed ConnectionStatus = "connFailed"
	ConnUp     ConnectionStatus = "connUp"
	ConnDown   ConnectionStatus = "connDown"
	ChanUp     ConnectionStatus = "chanUp"
	ChanDown   ConnectionStatus = "chanDown"
)

const (
	RedeliveryHeader = "x-redelivered-count"
	MaxRetryHeader   = "x-death"
)

// Connection Types.

type ConnectionType string

const (
	Consumer  ConnectionType = "consumer"
	Publisher ConnectionType = "publisher"
)

// Exchange Types.
const (
	TypeTopic   = "topic"
	TypeDirect  = "direct"
	Typeanout   = "fanout"
	TypeHeaders = "headers"
)

type MessagePriority uint8

// Priority Levels.
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

type DeliveryMode uint8

// Delivery Modes.
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
	errEmptyStringParse        = errors.New("could not parse empty string")
	errInvalidFormat           = errors.New("invalid format")
	errEmptyArgument           = errors.New("empty argument")
	errConnectionClosed        = errors.New("connection is closed")
	errChannelClosed           = errors.New("channel is closed")
	errConsumerAlreadyExists   = errors.New("consumer on declared queue already exists")
	errConsumerNotInitialized  = errors.New("consumer is not initialized")
	errPublisherNotInitialized = errors.New("publisher is not initialized")
)
