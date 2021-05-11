package gorabbit

const (
	TypeTopic   = "topic"
	TypeDirect  = "direct"
	TypeFanout  = "fanout"
	TypeHeaders = "headers"
)

const (
	SolangeEventQueue   = "solange_event_queue"
	SolangeCommandQueue = "solange_command_queue"

	ActivityEventQueue   = "activity_event_queue"
	ActivityCommandQueue = "activity_command_queue"

	PlanEventQueue   = "plan_event_queue"
	PlanCommandQueue = "plan_command_queue"
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
