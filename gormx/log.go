package gormx

import (
	"context"
	"errors"
	"fmt"
	"time"

	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"

	"github.com/jasonlabz/potato/log"
)

var (
	infoStr      = "[DB] %s\n[info] "
	warnStr      = "[DB] %s\n[warn] "
	errStr       = "[DB] %s\n[error] "
	traceStr     = "[DB] %s\n[%.3fms] [rows:%v] %s"
	traceWarnStr = "[DB] %s %s\n[%.3fms] [rows:%v] %s"
	traceErrStr  = "[DB] %s %s\n[%.3fms] [rows:%v] %s"
)

func NewLogger(config *gormLogger.Config) gormLogger.Interface {
	if config.Colorful {
		infoStr = gormLogger.Green + "%s\n" + gormLogger.Reset + gormLogger.Green + "[info] " + gormLogger.Reset
		warnStr = gormLogger.BlueBold + "%s\n" + gormLogger.Reset + gormLogger.Magenta + "[warn] " + gormLogger.Reset
		errStr = gormLogger.Magenta + "%s\n" + gormLogger.Reset + gormLogger.Red + "[error] " + gormLogger.Reset
		traceStr = gormLogger.Green + "%s\n" + gormLogger.Reset + gormLogger.Yellow + "[%.3fms] " + gormLogger.BlueBold + "[rows:%v]" + gormLogger.Reset + " %s"
		traceWarnStr = gormLogger.Green + "%s " + gormLogger.Yellow + "%s\n" + gormLogger.Reset + gormLogger.RedBold + "[%.3fms] " + gormLogger.Yellow + "[rows:%v]" + gormLogger.Magenta + " %s" + gormLogger.Reset
		traceErrStr = gormLogger.RedBold + "%s " + gormLogger.MagentaBold + "%s\n" + gormLogger.Reset + gormLogger.Yellow + "[%.3fms] " + gormLogger.BlueBold + "[rows:%v]" + gormLogger.Reset + " %s"
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
		log.GetCurrentGormLogger(ctx).Info(fmt.Sprintf(l.infoLogMsg+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...))
	}
}

// Warn print warn messages
func (l *logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Warn {
		log.GetCurrentGormLogger(ctx).Warn(fmt.Sprintf(l.warnLogMsg+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...))
	}
}

// Error print error messages
func (l *logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Error {
		log.GetCurrentGormLogger(ctx).Error(fmt.Sprintf(l.errLogMsg+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...))
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
			log.GetCurrentGormLogger(ctx).Error(fmt.Sprintf(l.traceErrLogMsg, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			log.GetCurrentGormLogger(ctx).Error(fmt.Sprintf(l.traceErrLogMsg, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormLogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			log.GetCurrentGormLogger(ctx).Warn(fmt.Sprintf(l.traceWarnLogMsg, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			log.GetCurrentGormLogger(ctx).Warn(fmt.Sprintf(l.traceWarnLogMsg, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	case l.LogLevel == gormLogger.Info:
		sql, rows := fc()
		if rows == -1 {
			log.GetCurrentGormLogger(ctx).Info(fmt.Sprintf(l.traceLogMsg, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, int64(-1), sql))
		} else {
			log.GetCurrentGormLogger(ctx).Info(fmt.Sprintf(l.traceLogMsg, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql))
		}
	}
}
