package log

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"potato/core/consts"
	"potato/core/utils"
)

var logField = []string{consts.ContextTraceID, consts.ContextUserID, consts.ContextRemoteAddr}

var defaultLoggerWrapper *LoggerWrapper

type LoggerWrapper struct {
	logger *zap.Logger
}

func ZapField(ctx context.Context, contextKey ...string) (fields []zap.Field) {
	for _, key := range contextKey {
		value := utils.GetString(ctx.Value(key))
		if value == "" {
			continue
		}
		fields = append(fields, zap.String(key, value))
	}
	return
}

func GetLogger(ctx context.Context) *LoggerWrapper {
	return &LoggerWrapper{
		logger: logger().With(ZapField(ctx, logField...)...),
	}
}

func GormLogger(ctx context.Context) *LoggerWrapper {
	return &LoggerWrapper{
		logger: logger().WithOptions(zap.AddCallerSkip(3)).
			With(ZapField(ctx, logField...)...),
	}
}

func (l *LoggerWrapper) WithContext(ctx context.Context) *LoggerWrapper {
	l.logger = l.logger.With(ZapField(ctx, logField...)...)
	return l
}

func (l *LoggerWrapper) WithError(err error) *LoggerWrapper {
	l.logger = l.logger.With(zap.Error(err))
	return l
}

func (l *LoggerWrapper) WithOption(opt zap.Option) *LoggerWrapper {
	l.logger = l.logger.WithOptions(opt)
	return l
}

func (l *LoggerWrapper) WithField(fields ...zap.Field) *LoggerWrapper {
	l.logger = l.logger.With(fields...)
	return l
}

func (l *LoggerWrapper) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

func (l *LoggerWrapper) Debugf(msg string, args ...any) {
	l.logger.Debug(fmt.Sprintf(msg, args...))
}

func (l *LoggerWrapper) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

func (l *LoggerWrapper) Infof(msg string, args ...any) {
	l.logger.Info(fmt.Sprintf(msg, args...))
}

func (l *LoggerWrapper) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

func (l *LoggerWrapper) Warnf(msg string, args ...any) {
	l.logger.Warn(fmt.Sprintf(msg, args...))
}

func (l *LoggerWrapper) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

func (l *LoggerWrapper) Errorf(msg string, args ...any) {
	l.logger.Error(fmt.Sprintf(msg, args...))
}

func (l *LoggerWrapper) Panic(msg string, fields ...zap.Field) {
	l.logger.Panic(msg, fields...)
}

func (l *LoggerWrapper) Panicf(msg string, args ...any) {
	l.logger.Panic(fmt.Sprintf(msg, args...))
}

func (l *LoggerWrapper) Fatal(msg string, fields ...zap.Field) {
	l.logger.Fatal(msg, fields...)
}

func (l *LoggerWrapper) Fatalf(msg string, args ...any) {
	l.logger.Fatal(fmt.Sprintf(msg, args...))
}

func (l *LoggerWrapper) Sync() {
	_ = l.logger.Sync()
}
