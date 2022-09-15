package gorabbit

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Logger is the interface that defines log methods.
type Logger interface {
	WithField(string, interface{}) Logger

	Error(error, string)

	Warn(string)

	Info(string)

	Debug(string)
}

// stdLogger logs to stdout using logrus (https://github.com/sirupsen/logrus).
type stdLogger struct {
	logger     *logrus.Logger
	identifier string
	logFields  map[string]interface{}
}

func newStdLogger() Logger {
	return &stdLogger{
		logger:     newLogrus(),
		identifier: libraryName,
		logFields:  nil,
	}
}

func (l stdLogger) WithField(key string, value interface{}) Logger {
	l.logger.WithField(key, value)

	return l
}

func (l stdLogger) Error(err error, s string) {
	l.logger.WithField("library", l.identifier).WithFields(l.logFields).WithError(err).Error(s)
}

func (l stdLogger) Warn(s string) {
	l.logger.WithField("library", l.identifier).WithFields(l.logFields).Warn(s)
}

func (l stdLogger) Info(s string) {
	l.logger.WithField("library", l.identifier).WithFields(l.logFields).Info(s)
}

func (l stdLogger) Debug(s string) {
	l.logger.WithField("library", l.identifier).WithFields(l.logFields).Debug(s)
}

// noLogger does not log at all, this is the default.
type noLogger struct{}

func (l noLogger) WithField(key string, value interface{}) Logger { return l }

func (l noLogger) Error(err error, s string) {}

func (l noLogger) Warn(s string) {}

func (l noLogger) Info(s string) {}

func (l noLogger) Debug(s string) {}

func newLogrus() *logrus.Logger {
	logger := &logrus.Logger{
		Out: os.Stdout,
		Formatter: &logrus.JSONFormatter{
			DisableTimestamp: true,
		},
		Level: logrus.DebugLevel,
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err == nil {
			logger.Level = lvl
		}
	}

	return logger
}

func inheritLogger(parent Logger, logFields map[string]interface{}) Logger {
	switch v := parent.(type) {
	case *stdLogger:
		return &stdLogger{
			logger:     v.logger,
			identifier: libraryName,
			logFields:  logFields,
		}
	default:
		return parent
	}
}
