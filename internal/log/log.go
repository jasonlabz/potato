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
	Info(ctx context.Context, msg string, fields ...any)
	Infof(ctx context.Context, msg string, args ...any)
	Debug(ctx context.Context, msg string, fields ...any)
	Debugf(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, fields ...any)
	Warnf(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, fields ...any)
	Errorf(ctx context.Context, msg string, args ...any)
	Sync()
}
