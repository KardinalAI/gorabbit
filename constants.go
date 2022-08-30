package gorabbit

import "errors"

type ConnectionStatus string

const (
	ConnFailed ConnectionStatus = "connFailed"
	ConnUp     ConnectionStatus = "connUp"
	ConnDown   ConnectionStatus = "connDown"
	ChanUp     ConnectionStatus = "chanUp"
	ChanDown   ConnectionStatus = "chanDown"
)

const (
	RedeliveryHeader = "x-redelivered-count"
)

// Exchange Types.
const (
	TypeTopic   = "topic"
	TypeDirect  = "direct"
	TypeFanout  = "fanout"
	TypeHeaders = "headers"
)

// Kardinal Specific.
const (
	EventsExchange   = "events_exchange"
	CommandsExchange = "commands_exchange"
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

// Mode.
const (
	Release = "release"
	Debug   = "debug"
)

// Errors.
var (
	errConnectionOrChannelClosed = errors.New("connection or channel closed")
	errEmptyURI                  = errors.New("amqp uri is empty")
	errDeliveryNotInitialized    = errors.New("delivery not initialized")
	errStringParse               = errors.New("could not parse empty string")
	errInvalidFormat             = errors.New("invalid format")
	errEmptyArgument             = errors.New("empty argument")
	errMaxRetryReached           = errors.New("max retry has been reached")
	errEmptyQueue                = errors.New("queue is empty")
)
