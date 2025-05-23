package log

import (
	"context"
	"fmt"
	"sync"

	"github.com/jasonlabz/potato/utils"
	"go.uber.org/zap"
)

type LoggerWrapper struct {
	logField []string
	logger   *zap.Logger
}

var defaultLogger *LoggerWrapper
var once sync.Once

func init() {
	once.Do(func() {
		defaultLogger = NewLogger()
	})
	if defaultLogger == nil {
		panic("init logger fail!")
	}
}

func GetLogger(opts ...zap.Option) *LoggerWrapper {
	if len(opts) > 0 {
		return defaultLogger.WithOptions(opts...)
	}
	return defaultLogger
}

func (l *LoggerWrapper) clone() *LoggerWrapper {
	cloneLogger := *l.logger
	return &LoggerWrapper{logger: &cloneLogger, logField: l.logField}
}

func (l *LoggerWrapper) WithContext(ctx context.Context) *LoggerWrapper {
	lw := l.clone()
	lw.logger = lw.logger.With(zapField(ctx, lw.logField...)...)
	return lw
}

func (l *LoggerWrapper) WithError(err error) *LoggerWrapper {
	lw := l.clone()
	lw.logger = lw.logger.With(String("error", err.Error()))
	return lw
}

// WithCallerSkip 调用层级
func (l *LoggerWrapper) WithCallerSkip(callerSkip int) *LoggerWrapper {
	lw := l.clone()
	lw.logger = lw.logger.WithOptions(zap.AddCallerSkip(callerSkip))
	return lw
}

func (l *LoggerWrapper) WithOptions(opt ...zap.Option) *LoggerWrapper {
	lw := l.clone()
	lw.logger = lw.logger.WithOptions(opt...)
	return lw
}

func (l *LoggerWrapper) WithField(fields ...zap.Field) *LoggerWrapper {
	lw := l.clone()
	lw.logger = lw.logger.With(fields...)
	return lw
}

func (l *LoggerWrapper) WithAny(fields ...any) *LoggerWrapper {
	lw := l.clone()
	lw.logger = lw.logger.With(lw.checkFields(fields)...)
	return lw
}

func (l *LoggerWrapper) Debug(msg string, fields ...any) {
	l.logger.Debug(msg, l.checkFields(fields)...)
}

func (l *LoggerWrapper) Debugf(msg string, args ...any) {
	l.logger.Debug(getMessage(msg, args))
}

func (l *LoggerWrapper) DebugContext(ctx context.Context, msg string, fields ...any) {
	field := zapField(ctx, l.logField...)
	checkFields := l.checkFields(fields)
	if len(checkFields) > 0 {
		field = append(field, checkFields...)
	}
	l.logger.Debug(msg, field...)
}

func (l *LoggerWrapper) Info(msg string, fields ...any) {
	l.logger.Info(msg, l.checkFields(fields)...)
}

func (l *LoggerWrapper) Infof(msg string, args ...any) {
	l.logger.Info(getMessage(msg, args))
}

func (l *LoggerWrapper) InfoContext(ctx context.Context, msg string, fields ...any) {
	field := zapField(ctx, l.logField...)
	checkFields := l.checkFields(fields)
	if len(checkFields) > 0 {
		field = append(field, checkFields...)
	}
	l.logger.Info(msg, field...)
}

func (l *LoggerWrapper) Warn(msg string, fields ...any) {
	l.logger.Warn(msg, l.checkFields(fields)...)
}

func (l *LoggerWrapper) Warnf(msg string, args ...any) {
	l.logger.Warn(getMessage(msg, args))
}

func (l *LoggerWrapper) WarnContext(ctx context.Context, msg string, fields ...any) {
	field := zapField(ctx, l.logField...)
	checkFields := l.checkFields(fields)
	if len(checkFields) > 0 {
		field = append(field, checkFields...)
	}
	l.logger.Warn(msg, field...)
}

func (l *LoggerWrapper) Error(msg string, fields ...any) {
	l.logger.Error(msg, l.checkFields(fields)...)
}

func (l *LoggerWrapper) Errorf(msg string, args ...any) {
	l.logger.Error(getMessage(msg, args))
}

func (l *LoggerWrapper) ErrorContext(ctx context.Context, msg string, fields ...any) {
	field := zapField(ctx, l.logField...)
	checkFields := l.checkFields(fields)
	if len(checkFields) > 0 {
		field = append(field, checkFields...)
	}
	l.logger.Error(msg, field...)
}

func (l *LoggerWrapper) Panic(msg string, fields ...any) {
	l.logger.Panic(msg, l.checkFields(fields)...)
}

func (l *LoggerWrapper) Panicf(msg string, args ...any) {
	l.logger.Panic(getMessage(msg, args))
}

func (l *LoggerWrapper) PanicContext(ctx context.Context, msg string, fields ...any) {
	field := zapField(ctx, l.logField...)
	checkFields := l.checkFields(fields)
	if len(checkFields) > 0 {
		field = append(field, checkFields...)
	}
	l.logger.Panic(msg, field...)
}

func (l *LoggerWrapper) Fatal(msg string, fields ...any) {
	l.logger.Fatal(msg, l.checkFields(fields)...)
}

func (l *LoggerWrapper) Fatalf(msg string, args ...any) {
	l.logger.Panic(getMessage(msg, args))
}

func (l *LoggerWrapper) FatalContext(ctx context.Context, msg string, fields ...any) {
	field := zapField(ctx, l.logField...)
	checkFields := l.checkFields(fields)
	if len(checkFields) > 0 {
		field = append(field, checkFields...)
	}
	l.logger.Fatal(msg, field...)
}

func (l *LoggerWrapper) Sync() {
	_ = l.logger.Sync()
}

func (l *LoggerWrapper) checkFields(fields []any) (checked []zap.Field) {
	checked = make([]zap.Field, 0)

	if len(fields) == 0 {
		return
	}

	_, isZapField := fields[0].(zap.Field)
	if isZapField {
		for _, field := range fields {
			if f, ok := field.(zap.Field); ok {
				checked = append(checked, f)
			}
		}
		return
	}

	if len(fields) == 1 {
		checked = append(checked, zap.Any("log_field", utils.StringValue(fields[0])))
		return
	}

	for i := 0; i < len(fields)-1; {
		checked = append(checked, zap.Any(utils.StringValue(fields[i]), utils.StringValue(fields[i+1])))
		if i == len(fields)-3 {
			checked = append(checked, zap.Any("log_field", utils.StringValue(fields[i+2])))
		}
		i += 2
	}

	return
}

// getMessage format with Sprint, Sprintf, or neither.
func getMessage(template string, args []any) string {
	if len(args) == 0 {
		return template
	}

	if template != "" {
		return fmt.Sprintf(template, args...)
	}

	if len(args) == 1 {
		if str, ok := args[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(args...)
}

func Any(key string, val any) zap.Field {
	return zap.Any(key, val)
}

func String(key, val string) zap.Field {
	return zap.String(key, val)
}

func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

func Int32(key string, val int32) zap.Field {
	return zap.Int32(key, val)
}

func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func Float64(key string, val float64) zap.Field {
	return zap.Float64(key, val)
}

func Float32(key string, val float32) zap.Field {
	return zap.Float32(key, val)
}

func zapField(ctx context.Context, contextKey ...string) (fields []zap.Field) {
	for _, key := range contextKey {
		value := utils.StringValue(ctx.Value(key))
		if value == "" {
			continue
		}
		fields = append(fields, zap.String(key, value))
	}
	return
}
