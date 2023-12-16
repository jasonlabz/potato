package gormx

import (
	"context"
	"errors"
	"fmt"
	"time"

	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"

	log "github.com/jasonlabz/potato/log/zapx"
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
	//logWrapper *log.LoggerWrapper
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
		log.GormLogger(ctx).Infof(l.infoLogMsg+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

// Warn print warn messages
func (l *logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Warn {
		log.GormLogger(ctx).Infof(l.warnLogMsg+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

// Error print error messages
func (l *logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Error {
		log.GormLogger(ctx).Infof(l.errLogMsg+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
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
			log.GormLogger(ctx).Infof(l.traceErrLogMsg, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql)
		} else {
			log.GormLogger(ctx).Infof(l.traceErrLogMsg, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormLogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			log.GormLogger(ctx).Infof(l.traceWarnLogMsg, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, int64(-1), sql)
		} else {
			log.GormLogger(ctx).Infof(l.traceWarnLogMsg, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case l.LogLevel == gormLogger.Info:
		sql, rows := fc()
		if rows == -1 {
			log.GormLogger(ctx).Infof(l.traceLogMsg, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, int64(-1), sql)
		} else {
			log.GormLogger(ctx).Infof(l.traceLogMsg, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}
