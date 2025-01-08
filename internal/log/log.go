// Package log -----------------------------
// @file      : log.go
// @author    : jasonlabz
// @contact   : 1783022886@qq.com
// @time      : 2024/12/10 0:54
// -------------------------------------------
package log

import (
	"context"
)

type Logger interface {
	Info(msg string, fields ...any)
	Infof(msg string, args ...any)
	InfoContext(ctx context.Context, msg string, fields ...any)
	Debug(msg string, fields ...any)
	Debugf(msg string, args ...any)
	DebugContext(ctx context.Context, msg string, fields ...any)
	Warn(msg string, fields ...any)
	Warnf(msg string, args ...any)
	WarnContext(ctx context.Context, msg string, fields ...any)
	Error(msg string, fields ...any)
	Errorf(msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, fields ...any)
	Sync()
}
