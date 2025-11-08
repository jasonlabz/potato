package slogx

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

func Test_Slog(t *testing.T) {
	slog.Info("hello")

	slog.Info("hello", slog.String("test", "dksjdjaks"))
}

func Test_Wrapper(t *testing.T) {
	ctx := context.Background()
	slog.Info(ctx, "sadasdasdasda")
	logger := GetLogger()
	logger.Info(ctx, "test", "hjhk", "sadasd")
	logger.Info(ctx, "test error", errors.New("test error"))
}
