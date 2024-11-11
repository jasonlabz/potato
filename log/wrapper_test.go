package log

import (
	"context"
	"go.uber.org/zap"
	"log/slog"
	"runtime/debug"
	"testing"
)

func TestName(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {

			DefaultLogger().WithField(zap.Any("error", err)).Error("[Recovery from panic]")
			DefaultLogger().Error(string(debug.Stack()))
		}
	}()
	ctx := context.TODO()
	GetLogger(ctx).Error("ttt%s,%s,%s", "sadas", "sdasd", "time")
	GetLogger(ctx).Panic("ttt%s,%s,%s", "sadas", "sdasd", "time")
	slog.Info("test")
}
