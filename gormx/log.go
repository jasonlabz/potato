package gormx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jasonlabz/potato/log"
	gormLogger "gorm.io/gorm/logger"
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
	return &logger{
		Config:          config,
		infoLogMsg:      infoStr,
		warnLogMsg:      warnStr,
		errLogMsg:       errStr,
		traceLogMsg:     traceStr,
		traceWarnLogMsg: traceWarnStr,
		traceErrLogMsg:  traceErrStr,
	}
}

type logger struct {
	//logWrapper *zapx.LoggerWrapper
	*gormLogger.Config
	infoLogMsg, warnLogMsg, errLogMsg            string
	traceLogMsg, traceErrLogMsg, traceWarnLogMsg string
}

// LogMode log mode
func (l *logger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info print info
func (l *logger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Info {
		log.GetGormLogger(ctx).Info(fmt.Sprintf(l.infoLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Warn print warn messages
func (l *logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Warn {
		log.GetGormLogger(ctx).Warn(fmt.Sprintf(l.warnLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Error print error messages
func (l *logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Error {
		log.GetGormLogger(ctx).Error(fmt.Sprintf(l.errLogMsg+msg, append([]interface{}{}, data...)...))
	}
}

// Trace print sql message
func (l *logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormLogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= gormLogger.Error && (!errors.Is(err, gormLogger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			log.GetGormLogger(ctx).Error(fmt.Sprintf(l.traceErrLogMsg, err, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			log.GetGormLogger(ctx).Error(fmt.Sprintf(l.traceErrLogMsg, err, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormLogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			log.GetGormLogger(ctx).Warn(fmt.Sprintf(l.traceWarnLogMsg, slowLog, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			log.GetGormLogger(ctx).Warn(fmt.Sprintf(l.traceWarnLogMsg, slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case l.LogLevel == gormLogger.Info:
		sql, rows := fc()
		if rows == -1 {
			log.GetGormLogger(ctx).Info(fmt.Sprintf(l.traceLogMsg, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			log.GetGormLogger(ctx).Info(fmt.Sprintf(l.traceLogMsg, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	}
}
