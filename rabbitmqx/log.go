package rabbitmqx

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jasonlabz/potato/internal/log"
)

type LoggerInterface interface {
	amqp.Logging
	Info(context.Context, string, ...interface{})
	Warn(context.Context, string, ...interface{})
	Error(context.Context, string, ...interface{})
}

func AdapterLogger(l log.Logger) *RmqLogger {
	return &RmqLogger{l: l}
}

type RmqLogger struct {
	l log.Logger
}

func (l *RmqLogger) Printf(msg string, args ...interface{}) {
	l.l.Info(msg, args...)
}

func (l *RmqLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	l.l.InfoContext(ctx, msg, args...)
}

func (l *RmqLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	l.l.WarnContext(ctx, msg, args...)
}

func (l *RmqLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	l.l.ErrorContext(ctx, msg, args...)
}
