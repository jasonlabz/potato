package zapx

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"strings"

	"go.uber.org/zap"

	"github.com/jasonlabz/potato/core/consts"
	"github.com/jasonlabz/potato/core/utils"
)

var logField = []string{consts.ContextTraceID, consts.ContextUserID, consts.ContextRemoteAddr}

type loggerWrapper struct {
	logger *zap.Logger
}

func zapField(ctx context.Context, contextKey ...string) (fields []zap.Field) {
	for _, key := range contextKey {
		value := utils.GetString(ctx.Value(key))
		if value == "" && key == consts.ContextTraceID {
			value = strings.ReplaceAll(uuid.New().String(), consts.SignDash, consts.EmptyString)
		}
		if value == "" {
			continue
		}
		fields = append(fields, zap.String(key, value))
	}
	return
}

func GetLogger(ctx context.Context) *loggerWrapper {
	return &loggerWrapper{
		logger: logger().With(zapField(ctx, logField...)...),
	}
}

func GormLogger(ctx context.Context) *loggerWrapper {
	return &loggerWrapper{
		logger: logger().WithOptions(zap.AddCallerSkip(3)).
			With(zapField(ctx, logField...)...),
	}
}

func (l *loggerWrapper) WithError(err error) *loggerWrapper {
	l.logger = l.logger.With(zap.Error(err))
	return l
}

func (l *loggerWrapper) WithOption(opt zap.Option) *loggerWrapper {
	l.logger = l.logger.WithOptions(opt)
	return l
}

func (l *loggerWrapper) WithField(fields ...any) *loggerWrapper {
	l.logger = l.logger.With(l.checkFields(fields)...)
	return l
}

func (l *loggerWrapper) Debug(msg string, fields ...any) {
	l.logger.Debug(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Debugf(msg string, args ...any) {
	l.logger.Debug(fmt.Sprintf(msg, args...))
}

func (l *loggerWrapper) Info(msg string, fields ...any) {
	l.logger.Info(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Infof(msg string, args ...any) {
	l.logger.Info(fmt.Sprintf(msg, args...))
}

func (l *loggerWrapper) Warn(msg string, fields ...any) {
	l.logger.Warn(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Warnf(msg string, args ...any) {
	l.logger.Warn(fmt.Sprintf(msg, args...))
}

func (l *loggerWrapper) Error(msg string, fields ...any) {
	l.logger.Error(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Errorf(msg string, args ...any) {
	l.logger.Error(fmt.Sprintf(msg, args...))
}

func (l *loggerWrapper) Panic(msg string, fields ...any) {
	l.logger.Panic(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Panicf(msg string, args ...any) {
	l.logger.Panic(fmt.Sprintf(msg, args...))
}

func (l *loggerWrapper) Fatal(msg string, fields ...any) {
	l.logger.Fatal(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Fatalf(msg string, args ...any) {
	l.logger.Fatal(fmt.Sprintf(msg, args...))
}

func (l *loggerWrapper) Sync() {
	_ = l.logger.Sync()
}

func (l *loggerWrapper) checkFields(fields ...any) (checked []zap.Field) {
	for _, field := range fields {
		if f, ok := field.(zap.Field); ok {
			checked = append(checked, f)
		}
	}
	return
}
