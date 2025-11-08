package slogx

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/jasonlabz/potato/syncer"
	"github.com/jasonlabz/potato/utils"
)

func init() {
	once.Do(func() {
		defaultLogger = NewLogger()
	})
	if defaultLogger == nil {
		panic("init logger fail!")
	}
}

var defaultLogger *LoggerWrapper
var once sync.Once

type LoggerWrapper struct {
	logger     *slog.Logger
	core       syncer.WriteSyncer
	logField   []string
	callerSkip int
}

func GetLogger() *LoggerWrapper {
	return defaultLogger.clone()
}

func slogField(ctx context.Context, contextKey ...string) (fields []any) {
	for _, key := range contextKey {
		value := utils.StringValue(ctx.Value(key))
		// if value == "" && key == consts.ContextTraceID {
		//	value = strings.ReplaceAll(uuid.New().String(), consts.SignDash, consts.EmptyString)
		// }
		if value == "" {
			continue
		}
		fields = append(fields, slog.String(key, value))
	}
	return
}

func (l *LoggerWrapper) clone() *LoggerWrapper {
	c := *l.logger
	return &LoggerWrapper{logger: &c, logField: l.logField, core: l.core, callerSkip: l.callerSkip}
}

// WithCallerSkip 调用层级
func (l *LoggerWrapper) WithCallerSkip(callerSkip int) *LoggerWrapper {
	l.callerSkip = callerSkip
	return l
}

func (l *LoggerWrapper) WithError(err error) *LoggerWrapper {
	l.logger = l.logger.With(Any("error", err))
	return l
}

func (l *LoggerWrapper) WithGroup(groupName string) *LoggerWrapper {
	l.logger = l.logger.WithGroup(groupName)
	return l
}

func (l *LoggerWrapper) WithAny(attrs ...any) *LoggerWrapper {
	l.logger = l.logger.With(attrs...)
	return l
}

// Debug logs at [LevelDebug].
func (l *LoggerWrapper) Debug(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, args...)
}

func (l *LoggerWrapper) Debugf(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, getMessage(msg, args))
}

// Info logs at [LevelInfo].
func (l *LoggerWrapper) Info(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, args...)
}

func (l *LoggerWrapper) Infof(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, getMessage(msg, args))
}

// Warn logs at [LevelWarn].
func (l *LoggerWrapper) Warn(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, args...)
}

func (l *LoggerWrapper) Warnf(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, getMessage(msg, args))
}

// Error logs at [LevelError].
func (l *LoggerWrapper) Error(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, msg, args...)
}

func (l *LoggerWrapper) Errorf(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, getMessage(msg, args))
}

func (l *LoggerWrapper) Sync() error {
	return l.core.Sync()
}

// log is the low-level logging method for methods that take ...any.
// It must always be called directly by an exported logging method
// or function, because it uses a fixed call depth to obtain the pc.
func (l *LoggerWrapper) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if !l.logger.Enabled(ctx, level) {
		return
	}
	fields := slogField(ctx, l.logField...)
	if len(args) > 0 {
		fields = append(fields, args...)
	}
	var pc uintptr
	var pcs [1]uintptr
	// skip [runtime.Callers, this function, this function's caller]
	runtime.Callers(l.callerSkip, pcs[:])
	pc = pcs[0]
	r := slog.NewRecord(time.Now(), level, msg, pc)
	r.Add(fields...)
	if ctx == nil {
		ctx = context.Background()
	}
	_ = l.logger.Handler().Handle(ctx, r)
}

// LogAttrs is a more efficient version of [Logger.Log] that accepts only Attrs.
func (l *LoggerWrapper) LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	l.logAttrs(ctx, level, msg, attrs...)
}

// logAttrs is like [Logger.log], but for methods that take ...Attr.
func (l *LoggerWrapper) logAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if !l.logger.Enabled(ctx, level) {
		return
	}
	var pc uintptr
	var pcs [1]uintptr
	// skip [runtime.Callers, this function, this function's caller]
	runtime.Callers(l.callerSkip, pcs[:])
	pc = pcs[0]
	r := slog.NewRecord(time.Now(), level, msg, pc)
	r.AddAttrs(attrs...)
	if ctx == nil {
		ctx = context.Background()
	}
	_ = l.logger.Handler().Handle(ctx, r)
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

func Any(key string, val any) slog.Attr {
	return slog.Any(key, val)
}

func String(key, val string) slog.Attr {
	return slog.String(key, val)
}

func Int64(key string, val int64) slog.Attr {
	return slog.Int64(key, val)
}

func Int32(key string, val int32) slog.Attr {
	return slog.Int(key, int(val))
}

func Int(key string, val int) slog.Attr {
	return slog.Int(key, val)
}

func Float64(key string, val float64) slog.Attr {
	return slog.Float64(key, val)
}

func Float32(key string, val float32) slog.Attr {
	return slog.Float64(key, float64(val))
}
