package log

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/jasonlabz/potato/utils"
)

type LoggerWrapper struct {
	logField []string
	logger   *zap.Logger
}

// badKey 与 slog 保持一致：参数不成对或 key 不是字符串时使用该字段名，
// 让调用错误在日志里可见，而不是被静默丢弃。
const badKey = "!BADKEY"

var defaultLogger atomic.Pointer[LoggerWrapper]

var initOnce sync.Once

func GetLogger(opts ...zap.Option) *LoggerWrapper {
	l := defaultLogger.Load()
	if l == nil {
		initOnce.Do(func() {
			// 允许初始化前通过 SetLogger 预先注入实例
			if defaultLogger.Load() == nil {
				defaultLogger.Store(NewLogger())
			}
		})
		l = defaultLogger.Load()
	}
	if l == nil {
		panic("init logger fail!")
	}

	if len(opts) > 0 {
		return l.WithOptions(opts...)
	}
	return l
}

// SetLogger 替换全局 logger，通常用于启动阶段把 NewLogger 的结果安装为全局实例，
// 避免框架内部包在 GetLogger 首次调用时就固定成默认配置。传 nil 时忽略。
func SetLogger(l *LoggerWrapper) {
	if l == nil {
		return
	}
	defaultLogger.Store(l)
}

func (l *LoggerWrapper) clone() *LoggerWrapper {
	cloneLogger := *l.logger
	return &LoggerWrapper{logger: &cloneLogger, logField: l.logField}
}

func (l *LoggerWrapper) ZapLogger() *zap.Logger {
	cloneLogger := *l.logger
	return &cloneLogger
}

func (l *LoggerWrapper) WithError(err error) *LoggerWrapper {
	if err == nil {
		return l
	}
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

func (l *LoggerWrapper) Debug(ctx context.Context, msg string, fields ...any) {
	l.logger.Debug(msg, l.prepareFields(ctx, fields)...)
}

func (l *LoggerWrapper) Debugf(ctx context.Context, msg string, args ...any) {
	l.logger.Debug(getMessage(msg, args), l.prepareFields(ctx, nil)...)
}

func (l *LoggerWrapper) Info(ctx context.Context, msg string, fields ...any) {
	l.logger.Info(msg, l.prepareFields(ctx, fields)...)
}

func (l *LoggerWrapper) Infof(ctx context.Context, msg string, args ...any) {
	l.logger.Info(getMessage(msg, args), l.prepareFields(ctx, nil)...)
}

func (l *LoggerWrapper) Warn(ctx context.Context, msg string, fields ...any) {
	l.logger.Warn(msg, l.prepareFields(ctx, fields)...)
}

func (l *LoggerWrapper) Warnf(ctx context.Context, msg string, args ...any) {
	l.logger.Warn(getMessage(msg, args), l.prepareFields(ctx, nil)...)
}

func (l *LoggerWrapper) Error(ctx context.Context, msg string, fields ...any) {
	l.logger.Error(msg, l.prepareFields(ctx, fields)...)
}

func (l *LoggerWrapper) Errorf(ctx context.Context, msg string, args ...any) {
	l.logger.Error(getMessage(msg, args), l.prepareFields(ctx, nil)...)
}

func (l *LoggerWrapper) Panic(ctx context.Context, msg string, fields ...any) {
	l.logger.Panic(msg, l.prepareFields(ctx, fields)...)
}

func (l *LoggerWrapper) Panicf(ctx context.Context, msg string, args ...any) {
	l.logger.Panic(getMessage(msg, args), l.prepareFields(ctx, nil)...)
}

func (l *LoggerWrapper) Fatal(ctx context.Context, msg string, fields ...any) {
	l.logger.Fatal(msg, l.prepareFields(ctx, fields)...)
}

func (l *LoggerWrapper) Fatalf(ctx context.Context, msg string, args ...any) {
	l.logger.Fatal(getMessage(msg, args), l.prepareFields(ctx, nil)...)
}

func (l *LoggerWrapper) Sync() {
	_ = l.logger.Sync()
}

func (l *LoggerWrapper) prepareFields(ctx context.Context, fields []any) (field []zap.Field) {
	field = zapField(ctx, l.logField...)
	if checkFields := l.checkFields(fields); len(checkFields) > 0 {
		field = append(field, checkFields...)
	}
	return
}

// checkFields 把可变参数规范化为 zap.Field，支持三种写法并可任意混用：
//   - zap.Field：log.String/log.Any 等辅助函数返回的字段
//   - (string, any)：键值对
//   - error：以 error 字段输出
//
// 值不做字符串化，保留原始类型，避免数字/布尔在日志系统中退化为字符串。
// key 不是字符串、或字符串后面没有配对值、以及错把 zap.Field 当作值时，
// 记为 !BADKEY，让调用错误可见。
func (l *LoggerWrapper) checkFields(fields []any) (checked []zap.Field) {
	if len(fields) == 0 {
		return nil
	}

	checked = make([]zap.Field, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		switch v := fields[i].(type) {
		case zap.Field:
			checked = append(checked, v)
		case error:
			if v != nil {
				checked = append(checked, zap.Error(v))
			}
		case string:
			if i+1 >= len(fields) {
				checked = append(checked, zap.String(badKey, v))
				continue
			}
			val := fields[i+1]
			i++
			switch tv := val.(type) {
			case zap.Field:
				// 键被忽略，字段本身仍然有效，避免把它序列化成 Field 结构体
				checked = append(checked, tv)
			case error:
				checked = append(checked, zap.NamedError(v, tv))
			default:
				checked = append(checked, zap.Any(v, val))
			}
		default:
			checked = append(checked, zap.Any(badKey, v))
		}
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
	if ctx == nil || len(contextKey) == 0 {
		return nil
	}
	fields = make([]zap.Field, 0, len(contextKey))
	for _, key := range contextKey {
		value := utils.StringValue(ctx.Value(key))
		if value == "" {
			continue
		}
		fields = append(fields, zap.String(key, value))
	}
	return
}
