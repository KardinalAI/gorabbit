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
	MessagePriorityLowest  MessagePriority = 1
	MessagePriorityVeryLow MessagePriority = 2
	MessagePriorityLow     MessagePriority = 3
	MessagePriorityMedium  MessagePriority = 4
	MessagePriorityHigh    MessagePriority = 5
	MessagePriorityHighest MessagePriority = 6
)

func (m MessagePriority) Uint8() uint8 {
	return uint8(m)
}

// Mode
const (
	Release = "release"
	Debug   = "debug"
)
