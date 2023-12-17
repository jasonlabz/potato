package zapx

import (
	"context"
	"strings"

	"github.com/google/uuid"
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

func (l *loggerWrapper) Any(key string, val any) zap.Field {
	return zap.Any(key, val)
}

func (l *loggerWrapper) String(key, val string) zap.Field {
	return zap.String(key, val)
}

func (l *loggerWrapper) Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

func (l *loggerWrapper) Int32(key string, val int32) zap.Field {
	return zap.Int32(key, val)
}

func (l *loggerWrapper) Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func (l *loggerWrapper) Float64(key string, val float64) zap.Field {
	return zap.Float64(key, val)
}

func (l *loggerWrapper) Float32(key string, val float32) zap.Field {
	return zap.Float32(key, val)
}

func (l *loggerWrapper) Debug(msg string, fields ...any) {
	l.logger.Debug(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Info(msg string, fields ...any) {
	l.logger.Info(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Warn(msg string, fields ...any) {
	l.logger.Warn(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Error(msg string, fields ...any) {
	l.logger.Error(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Panic(msg string, fields ...any) {
	l.logger.Panic(msg, l.checkFields(fields)...)
}

func (l *loggerWrapper) Fatal(msg string, fields ...any) {
	l.logger.Fatal(msg, l.checkFields(fields)...)
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
