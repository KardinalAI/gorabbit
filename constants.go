package gorabbit

import (
	"errors"
	"time"
)

// Library name.
const libraryName = "Gorabbit"

// Default values for the ClientOptions and ManagerOptions.
const (
	defaultHost                = "127.0.0.1"
	defaultPort                = 5672
	defaultUsername            = "guest"
	defaultPassword            = "guest"
	defaultKeepAlive           = true
	defaultRetryDelay          = 3 * time.Second
	defaultMaxRetry            = 5
	defaultPublishingCacheTTL  = 60 * time.Second
	defaultPublishingCacheSize = 128
	defaultMode                = Release
)

const (
	xDeathCountHeader = "x-death-count"
)

// Connection Types.

type connectionType string

const (
	connectionTypeConsumer  connectionType = "consumer"
	connectionTypePublisher connectionType = "publisher"
)

// Exchange Types

type ExchangeType string

const (
	ExchangeTypeTopic   ExchangeType = "topic"
	ExchangeTypeDirect  ExchangeType = "direct"
	ExchangeTypeFanout  ExchangeType = "fanout"
	ExchangeTypeHeaders ExchangeType = "headers"
)

func (e ExchangeType) String() string {
	return string(e)
}

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
	errEmptyURI                          = errors.New("amqp uri is empty")
	errChannelClosed                     = errors.New("channel is closed")
	errConnectionClosed                  = errors.New("connection is closed")
	errConsumerAlreadyExists             = errors.New("consumer already exists")
	errConsumerConnectionNotInitialized  = errors.New("consumerConnection is not initialized")
	errPublisherConnectionNotInitialized = errors.New("publisherConnection is not initialized")
	errEmptyQueue                        = errors.New("queue is empty")
)
