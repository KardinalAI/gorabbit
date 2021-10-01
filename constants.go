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

// Priority Levels
const (
	PriorityLowest  = 1
	PriorityVeryLow = 2
	PriorityLow     = 3
	PriorityMedium  = 4
	PriorityHigh    = 5
	PriorityHighest = 6
)
