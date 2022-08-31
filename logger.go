package gorabbit

import (
	"gitlab.kardinal.ai/coretech/go-logging"
)

// Logger is the interface to send logs to. It can be set using
// WithPublisherOptionsLogger() or WithConsumerOptionsLogger().
type Logger interface {
	Printf(string, ...interface{})
}

const loggingPrefix = "Gorabbit"

// stdLogger logs to stdout using go's default logger.
type stdLogger struct{}

func (l stdLogger) Printf(format string, v ...interface{}) {
	logging.Logger.WithField("library", loggingPrefix).Infof(format, v...)
}

// noLogger does not log at all, this is the default.
type noLogger struct{}

func (l noLogger) Printf(format string, v ...interface{}) {}
