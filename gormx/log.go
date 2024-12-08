package gormx

import (
	"context"
	"errors"
	"fmt"
	"time"

	gormLogger "gorm.io/gorm/logger"

	"github.com/jasonlabz/potato/log"
)

var (
	infoStr                 = "[DB] [info] "
	warnStr                 = "[DB] [warn] "
	errStr                  = "[DB] [error] "
	traceStr                = "[DB] [%.3fms] [rows:%v] %s"
	traceWarnStr            = "[DB] %s [%.3fms] [rows:%v] %s"
	traceErrStr             = "[DB] %s [%.3fms] [rows:%v] %s"
	defaultGormLoggerConfig = &gormLogger.Config{
		SlowThreshold:             time.Second,       // Slow SQL threshold
		LogLevel:                  gormLogger.Silent, // Log level
		IgnoreRecordNotFoundError: false,             // Ignore ErrRecordNotFound error for logger
		Colorful:                  false,             // Disable color
	}
)

type OptionFunc func(*gormLogger.Config)

func WithLevel(level gormLogger.LogLevel) OptionFunc {
	return func(config *gormLogger.Config) {
		config.LogLevel = level
	}
}

func WithColorful(colorful bool) OptionFunc {
	return func(config *gormLogger.Config) {
		config.Colorful = colorful
	}
}

func WithIgnoreRecordNotFoundError(ignore bool) OptionFunc {
	return func(config *gormLogger.Config) {
		config.IgnoreRecordNotFoundError = ignore
	}
}

func WithSlowThreshold(threshold time.Duration) OptionFunc {
	return func(config *gormLogger.Config) {
		config.SlowThreshold = threshold
	}
}

func WithParameterizedQueries(query bool) OptionFunc {
	return func(config *gormLogger.Config) {
		config.ParameterizedQueries = query
	}
}

func LoggerAdapter(l *log.LoggerWrapper, opts ...OptionFunc) gormLogger.Interface {
	cloneConfig := *defaultGormLoggerConfig
	for _, opt := range opts {
		opt(&cloneConfig)
	}
	if cloneConfig.Colorful {
		infoStr = "[DB] " + gormLogger.Green + "[info] " + gormLogger.Reset
		warnStr = "[DB] " + gormLogger.Magenta + "[warn] " + gormLogger.Reset
		errStr = "[DB] " + gormLogger.Red + "[error] " + gormLogger.Reset
		traceStr = "[DB] " + gormLogger.Yellow + "[%.3fms] " + gormLogger.BlueBold + "[rows:%v]" + gormLogger.Reset + " %s"
		traceWarnStr = "[DB] " + gormLogger.Yellow + "%s\n" + gormLogger.Reset + gormLogger.RedBold + "[%.3fms] " + gormLogger.Yellow + "[rows:%v]" + gormLogger.Magenta + " %s" + gormLogger.Reset
		traceErrStr = "[DB] " + gormLogger.MagentaBold + "%s\n" + gormLogger.Reset + gormLogger.Yellow + "[%.3fms] " + gormLogger.BlueBold + "[rows:%v]" + gormLogger.Reset + " %s"
	}
	return &Logger{
		l:               l,
		Config:          &cloneConfig,
		infoLogMsg:      infoStr,
		warnLogMsg:      warnStr,
		errLogMsg:       errStr,
		traceLogMsg:     traceStr,
		traceWarnLogMsg: traceWarnStr,
		traceErrLogMsg:  traceErrStr,
	}
}

type Logger struct {
	*gormLogger.Config
	l                                            *log.LoggerWrapper
	infoLogMsg, warnLogMsg, errLogMsg            string
	traceLogMsg, traceErrLogMsg, traceWarnLogMsg string
}

// LogMode log mode
func (l *Logger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info print info
func (l *Logger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Info {
		l.l.InfoContext(ctx, fmt.Sprintf(l.infoLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Warn print warn messages
func (l *Logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Warn {
		l.l.WarnContext(ctx, fmt.Sprintf(l.warnLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Error print error messages
func (l *Logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Error {
		l.l.ErrorContext(ctx, fmt.Sprintf(l.errLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Trace print sql message
func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormLogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= gormLogger.Error && (!errors.Is(err, gormLogger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			l.l.ErrorContext(ctx, fmt.Sprintf(l.traceErrLogMsg, err, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			l.l.ErrorContext(ctx, fmt.Sprintf(l.traceErrLogMsg, err, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormLogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			l.l.WarnContext(ctx, fmt.Sprintf(l.traceWarnLogMsg, slowLog, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			l.l.WarnContext(ctx, fmt.Sprintf(l.traceWarnLogMsg, slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case l.LogLevel == gormLogger.Info:
		sql, rows := fc()
		if rows == -1 {
			l.l.InfoContext(ctx, fmt.Sprintf(l.traceLogMsg, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			l.l.InfoContext(ctx, fmt.Sprintf(l.traceLogMsg, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	}
}
