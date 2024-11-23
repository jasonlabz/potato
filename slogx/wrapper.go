package slogx

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jasonlabz/potato/consts"
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
	return &LoggerWrapper{
		logger:     defaultLogger.logger,
		core:       defaultLogger.core,
		logField:   defaultLogger.logField,
		callerSkip: defaultLogger.callerSkip,
	}
}

func slogField(ctx context.Context, contextKey ...string) (fields []any) {
	for _, key := range contextKey {
		value := utils.StringValue(ctx.Value(key))
		if value == "" && key == consts.ContextTraceID {
			value = strings.ReplaceAll(uuid.New().String(), consts.SignDash, consts.EmptyString)
		}
		if value == "" {
			continue
		}
		fields = append(fields, slog.String(key, value))
	}
	return
}

// WithCallerSkip 调用层级
func (l *LoggerWrapper) WithCallerSkip(callerSkip int) *LoggerWrapper {
	l.callerSkip = callerSkip
	return l
}

func (l *LoggerWrapper) WithContext(ctx context.Context) *LoggerWrapper {
	l.logger = l.logger.With(slogField(ctx, l.logField...)...)
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
func (l *LoggerWrapper) Debug(msg string, args ...any) {
	l.log(context.Background(), slog.LevelDebug, msg, args...)
}

// DebugContext logs at [LevelDebug] with the given context.
func (l *LoggerWrapper) DebugContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, args...)
}

// Info logs at [LevelInfo].
func (l *LoggerWrapper) Info(msg string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, msg, args...)
}

// InfoContext logs at [LevelInfo] with the given context.
func (l *LoggerWrapper) InfoContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, args...)
}

// Warn logs at [LevelWarn].
func (l *LoggerWrapper) Warn(msg string, args ...any) {
	l.log(context.Background(), slog.LevelWarn, msg, args...)
}

// WarnContext logs at [LevelWarn] with the given context.
func (l *LoggerWrapper) WarnContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, args...)
}

// Error logs at [LevelError].
func (l *LoggerWrapper) Error(msg string, args ...any) {
	l.log(context.Background(), slog.LevelError, msg, args...)
}

// ErrorContext logs at [LevelError] with the given context.
func (l *LoggerWrapper) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, msg, args...)
}

// log is the low-level logging method for methods that take ...any.
// It must always be called directly by an exported logging method
// or function, because it uses a fixed call depth to obtain the pc.
func (l *LoggerWrapper) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if !l.logger.Enabled(ctx, level) {
		return
	}
	var pc uintptr
	var pcs [1]uintptr
	// skip [runtime.Callers, this function, this function's caller]
	runtime.Callers(l.callerSkip, pcs[:])
	pc = pcs[0]
	r := slog.NewRecord(time.Now(), level, msg, pc)
	r.Add(args...)
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
