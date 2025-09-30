package rabbitmqx

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jasonlabz/potato/internal/log"
)

type LoggerInterface interface {
	amqp.Logging
	Info(context.Context, string, ...any)
	Warn(context.Context, string, ...any)
	Error(context.Context, string, ...any)
}

func AdapterLogger(l log.Logger) *RmqLogger {
	return &RmqLogger{l: l}
}

type RmqLogger struct {
	l log.Logger
}

func (l *RmqLogger) Printf(msg string, args ...any) {
	l.l.Info(msg, args...)
}

func (l *RmqLogger) Info(ctx context.Context, msg string, args ...any) {
	l.l.InfoContext(ctx, msg, args...)
}

func (l *RmqLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.l.WarnContext(ctx, msg, args...)
}

func (l *RmqLogger) Error(ctx context.Context, msg string, args ...any) {
	l.l.ErrorContext(ctx, msg, args...)
}
