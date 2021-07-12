package logging

import (
	"github.com/sirupsen/logrus"
	"os"
)

var Logger logrus.Logger

type LogFields = map[string]interface{}

func init() {
	var formatter = new(logrus.TextFormatter)
	formatter.DisableTimestamp = true

	Logger = logrus.Logger{
		Out:       os.Stdout,
		Formatter: formatter,
		Level:     logrus.DebugLevel,
	}
}
