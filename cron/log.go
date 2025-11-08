package xcron

import (
	"context"
	"fmt"

	"github.com/jasonlabz/potato/internal/log"
)

type CronLogger struct {
	l log.Logger
}

func (l CronLogger) Info(msg string, keysAndValues ...any) {
	l.l.Info(context.Background(), msg, keysAndValues...)
}

func (l CronLogger) Error(err error, msg string, keysAndValues ...any) {
	l.l.Error(context.Background(), msg+"[error] --> "+fmt.Sprint(err), keysAndValues...)
}

func AdapterCronLogger(l log.Logger) *CronLogger {
	return &CronLogger{l: l}
}
