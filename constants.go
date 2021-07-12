package gorabbit

const (
	RedeliveryHeader = "x-redelivered-count"
)

const (
	TypeTopic   = "topic"
	TypeDirect  = "direct"
	TypeFanout  = "fanout"
	TypeHeaders = "headers"
)

const (
	EventsExchange   = "events_exchange"
	CommandsExchange = "commands_exchange"
)

const (
	PriorityLowest  = 1
	PriorityVeryLow = 2
	PriorityLow     = 3
	PriorityMedium  = 4
	PriorityHigh    = 5
	PriorityHighest = 6
	PriorityUrgent  = 7
)
