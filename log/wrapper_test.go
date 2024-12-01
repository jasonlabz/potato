package log

import (
	"context"
	"runtime/debug"
	"testing"
)

func TestName(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {

			GetLogger().WithField(Any("error", err)).Error("[Recovery from panic]")
			GetLogger().Error(string(debug.Stack()))
		}
	}()
	ctx := context.TODO()
	GetLogger().WithContext(ctx).Error("ttt%s,%s,%s", "sadas", "sdasd", "time")
	//GetLogger().WithContext(ctx).Panic("ttt%s,%s,%s", "sadas", "sdasd", "time")
	//slog.Info("test")

	Info(ctx, "hello", "sadas", "sdasd")
}
