package log

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/jasonlabz/potato/core/utils"
)

type loggerWrapper struct {
	logField []string
	logger   *zap.Logger
}

var defaultLogger *loggerWrapper
var once sync.Once

func init() {
	once.Do(func() {
		defaultLogger = newLogger()
	})
	if defaultLogger == nil {
		panic("init logger fail!")
	}
}

func DefaultLogger(opts ...zap.Option) *loggerWrapper {
	if len(opts) > 0 {
		return defaultLogger.WithOptions(opts...)
	}
	return defaultLogger
}

func GetLogger(ctx context.Context, opts ...zap.Option) *loggerWrapper {
	return utils.IsTrueOrNot(len(opts) > 0,
		&loggerWrapper{logger: defaultLogger.logger.WithOptions(opts...).With(zapField(ctx, defaultLogger.logField...)...)},
		&loggerWrapper{logger: defaultLogger.logger.With(zapField(ctx, defaultLogger.logField...)...)},
	)
}

func GormLogger(ctx context.Context, opts ...zap.Option) *loggerWrapper {
	return utils.IsTrueOrNot(len(opts) > 0,
		&loggerWrapper{logger: defaultLogger.logger.WithOptions(opts...).WithOptions(zap.AddCallerSkip(3)).With(zapField(ctx, defaultLogger.logField...)...)},
		&loggerWrapper{logger: defaultLogger.logger.WithOptions(zap.AddCallerSkip(3)).With(zapField(ctx, defaultLogger.logField...)...)},
	)
}

func (l *loggerWrapper) WithError(err error) *loggerWrapper {
	l.logger = l.logger.With(zap.Error(err))
	return l
}

func (l *loggerWrapper) WithOptions(opt ...zap.Option) *loggerWrapper {
	l.logger = l.logger.WithOptions(opt...)
	return l
}

func (l *loggerWrapper) WithField(fields ...zap.Field) *loggerWrapper {
	l.logger = l.logger.With(fields...)
	return l
}

func (l *loggerWrapper) WithAny(fields ...any) *loggerWrapper {
	l.logger = l.logger.With(l.checkFields(fields)...)
	return l
}

func (l *loggerWrapper) Debug(msg string, args ...any) {
	l.logger.Debug(getMessage(msg, args))
}

func (l *loggerWrapper) Info(msg string, args ...any) {
	l.logger.Info(getMessage(msg, args))
}

func (l *loggerWrapper) Warn(msg string, args ...any) {
	l.logger.Warn(getMessage(msg, args))
}

func (l *loggerWrapper) Error(msg string, args ...any) {
	l.logger.Error(getMessage(msg, args))
}

func (l *loggerWrapper) Panic(msg string, args ...any) {
	l.logger.Panic(getMessage(msg, args))
}

func (l *loggerWrapper) Fatal(msg string, args ...any) {
	l.logger.Fatal(getMessage(msg, args))
}

func (l *loggerWrapper) Sync() {
	_ = l.logger.Sync()
}

func (l *loggerWrapper) checkFields(fields []any) (checked []zap.Field) {
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
		checked = append(checked, zap.Any("log_field", utils.GetString(fields[0])))
		return
	}

	for i := 0; i < len(fields)-1; {
		checked = append(checked, zap.Any(utils.GetString(fields[i]), utils.GetString(fields[i+1])))
		if i == len(fields)-3 {
			checked = append(checked, zap.Any("log_field", utils.GetString(fields[i+2])))
		}
		i += 2
	}

	return
}

// getMessage format with Sprint, Sprintf, or neither.
func getMessage(template string, fmtArgs []interface{}) string {
	if len(fmtArgs) == 0 {
		return template
	}

	if template != "" {
		return fmt.Sprintf(template, fmtArgs...)
	}

	if len(fmtArgs) == 1 {
		if str, ok := fmtArgs[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(fmtArgs...)
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
		value := utils.GetString(ctx.Value(key))
		if value == "" {
			continue
		}
		fields = append(fields, zap.String(key, value))
	}
	return
}
