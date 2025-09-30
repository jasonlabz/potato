package xcron

import (
	"fmt"

	"github.com/jasonlabz/potato/internal/log"
)

type CronLogger struct {
	l log.Logger
}

func (l CronLogger) Info(msg string, keysAndValues ...any) {
	l.l.Info(msg, keysAndValues...)
}

func (l CronLogger) Error(err error, msg string, keysAndValues ...any) {
	l.l.Error(msg+"[error] --> "+fmt.Sprint(err), keysAndValues...)
}

func AdapterCronLogger(l log.Logger) *CronLogger {
	return &CronLogger{l: l}
}
