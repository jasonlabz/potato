package gormx

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"time"

	gormLogger "gorm.io/gorm/logger"

	"github.com/jasonlabz/potato/log"
)

var (
	infoStr      = "[DB] [info] "
	warnStr      = "[DB] [warn] "
	errStr       = "[DB] [error] "
	traceStr     = "[DB] [%.3fms] [rows:%v] %s"
	traceWarnStr = "[DB] %s [%.3fms] [rows:%v] %s"
	traceErrStr  = "[DB] %s [%.3fms] [rows:%v] %s"
)

func NewLogger(config *gormLogger.Config) gormLogger.Interface {
	if config.Colorful {
		infoStr = "[DB] " + gormLogger.Green + "[info] " + gormLogger.Reset
		warnStr = "[DB] " + gormLogger.Magenta + "[warn] " + gormLogger.Reset
		errStr = "[DB] " + gormLogger.Red + "[error] " + gormLogger.Reset
		traceStr = "[DB] " + gormLogger.Yellow + "[%.3fms] " + gormLogger.BlueBold + "[rows:%v]" + gormLogger.Reset + " %s"
		traceWarnStr = "[DB] " + gormLogger.Yellow + "%s\n" + gormLogger.Reset + gormLogger.RedBold + "[%.3fms] " + gormLogger.Yellow + "[rows:%v]" + gormLogger.Magenta + " %s" + gormLogger.Reset
		traceErrStr = "[DB] " + gormLogger.MagentaBold + "%s\n" + gormLogger.Reset + gormLogger.Yellow + "[%.3fms] " + gormLogger.BlueBold + "[rows:%v]" + gormLogger.Reset + " %s"
	}
	return &Logger{
		Config:          config,
		infoLogMsg:      infoStr,
		warnLogMsg:      warnStr,
		errLogMsg:       errStr,
		traceLogMsg:     traceStr,
		traceWarnLogMsg: traceWarnStr,
		traceErrLogMsg:  traceErrStr,
	}
}

type Logger struct {
	//logWrapper *zapx.LoggerWrapper
	*gormLogger.Config
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
		log.GetLogger(zap.AddCallerSkip(3)).WithContext(ctx).Info(fmt.Sprintf(l.infoLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Warn print warn messages
func (l *Logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Warn {
		log.GetLogger(zap.AddCallerSkip(3)).WithContext(ctx).Warn(fmt.Sprintf(l.warnLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Error print error messages
func (l *Logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Error {
		log.GetLogger(zap.AddCallerSkip(3)).WithContext(ctx).Error(fmt.Sprintf(l.errLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Trace print sql message
func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormLogger.Silent {
		return
	}
	logger := log.GetLogger(zap.AddCallerSkip(3)).WithContext(ctx)
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= gormLogger.Error && (!errors.Is(err, gormLogger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			logger.Error(fmt.Sprintf(l.traceErrLogMsg, err, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			logger.Error(fmt.Sprintf(l.traceErrLogMsg, err, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormLogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			logger.Warn(fmt.Sprintf(l.traceWarnLogMsg, slowLog, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			logger.Warn(fmt.Sprintf(l.traceWarnLogMsg, slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case l.LogLevel == gormLogger.Info:
		sql, rows := fc()
		if rows == -1 {
			logger.Info(fmt.Sprintf(l.traceLogMsg, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			logger.Info(fmt.Sprintf(l.traceLogMsg, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	}
}
