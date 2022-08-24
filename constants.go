package gorabbit

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

// Exchange Types
const (
	TypeTopic   = "topic"
	TypeDirect  = "direct"
	TypeFanout  = "fanout"
	TypeHeaders = "headers"
)

// Kardinal Specific
const (
	EventsExchange   = "events_exchange"
	CommandsExchange = "commands_exchange"
)

type MessagePriority uint8

// Priority Levels
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

// Mode
const (
	Release = "release"
	Debug   = "debug"
)
