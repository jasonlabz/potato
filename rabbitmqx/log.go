package rabbitmqx

import (
	"context"
	"fmt"
	log "github.com/jasonlabz/potato/log/zapx"
	amqp "github.com/rabbitmq/amqp091-go"
)

type LoggerInterface interface {
	amqp.Logging
	Info(context.Context, string, ...interface{})
	Warn(context.Context, string, ...interface{})
	Error(context.Context, string, ...interface{})
}

var defaultLogger rmqLogger = rmqLogger{}

func getLogger() rmqLogger {
	return defaultLogger
}

type rmqLogger struct{}

func (l rmqLogger) Printf(msg string, args ...interface{}) {
	log.GetLogger(context.Background()).Info(fmt.Sprintf(msg, args))
}

func (l rmqLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	log.GetLogger(ctx).Info(fmt.Sprintf(msg, args))
}

func (l rmqLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	log.GetLogger(ctx).Warn(fmt.Sprintf(msg, args))
}

func (l rmqLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	log.GetLogger(ctx).Error(fmt.Sprintf(msg, args))
}
