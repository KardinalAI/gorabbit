package gorabbit

import (
	"os"

	"github.com/sirupsen/logrus"
)

type logField struct {
	Key   string
	Value interface{}
}

// logger is the interface that defines log methods.
type logger interface {
	Error(error, string, ...logField)

	Warn(string, ...logField)

	Info(string, ...logField)

	Debug(string, ...logField)
}

// stdLogger logs to stdout using logrus (https://github.com/sirupsen/logrus).
type stdLogger struct {
	logger     *logrus.Logger
	identifier string
	logFields  map[string]interface{}
}

func newStdLogger() logger {
	return &stdLogger{
		logger:     newLogrus(),
		identifier: libraryName,
		logFields:  nil,
	}
}

func (l stdLogger) getExtraFields(fields []logField) map[string]interface{} {
	extraFields := make(map[string]interface{})

	for k, field := range l.logFields {
		extraFields[k] = field
	}

	for _, extraField := range fields {
		extraFields[extraField.Key] = extraField.Value
	}

	return extraFields
}

func (l stdLogger) Error(err error, s string, fields ...logField) {
	log := l.logger.WithField("library", l.identifier)

	extraFields := l.getExtraFields(fields)

	log.WithFields(extraFields).WithError(err).Error(s)
}

func (l stdLogger) Warn(s string, fields ...logField) {
	log := l.logger.WithField("library", l.identifier)

	extraFields := l.getExtraFields(fields)

	log.WithFields(extraFields).Warn(s)
}

func (l stdLogger) Info(s string, fields ...logField) {
	log := l.logger.WithField("library", l.identifier)

	extraFields := l.getExtraFields(fields)

	log.WithFields(extraFields).Info(s)
}

func (l stdLogger) Debug(s string, fields ...logField) {
	log := l.logger.WithField("library", l.identifier)

	extraFields := l.getExtraFields(fields)

	log.WithFields(extraFields).Debug(s)
}

// noLogger does not log at all, this is the default.
type noLogger struct{}

func (l noLogger) Error(_ error, _ string, _ ...logField) {}

func (l noLogger) Warn(_ string, _ ...logField) {}

func (l noLogger) Info(_ string, _ ...logField) {}

func (l noLogger) Debug(_ string, _ ...logField) {}

func newLogrus() *logrus.Logger {
	log := &logrus.Logger{
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
			log.Level = lvl
		}
	}

	return log
}

func inheritLogger(parent logger, logFields map[string]interface{}) logger {
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
