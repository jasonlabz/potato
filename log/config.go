package log

import (
	"context"
	"github.com/jasonlabz/potato/log/slogx"
	"github.com/jasonlabz/potato/log/zapx"
)

type PotatoLogger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
	Debug(string, ...any)
}

type LoggerType string

const (
	LoggerTypeSlog = "slog"
	LoggerTypeZap  = "zap"
)

// 默认使用zap打印日志，可设置slog 通过SetCurrentLogger
var currentLogger LoggerType = LoggerTypeZap

func SetCurrentLogger(loggerType LoggerType) {
	currentLogger = loggerType
}

func GetCurrentLogger(ctx context.Context) PotatoLogger {
	switch currentLogger {
	case LoggerTypeZap:
		return zapx.GetLogger(ctx)
	case LoggerTypeSlog:
		return slogx.GetLogger(ctx)
	default:
		panic("undefined log type")
	}
}

func GetCurrentGormLogger(ctx context.Context) PotatoLogger {
	switch currentLogger {
	case LoggerTypeZap:
		return zapx.GormLogger(ctx)
	case LoggerTypeSlog:
		return slogx.GormLogger(ctx)
	default:
		panic("undefined log type")
	}
}
