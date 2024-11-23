package slogx

import (
	"context"
	"log/slog"
	"testing"
)

func Test_Slog(t *testing.T) {
	slog.Info("hello")

	slog.Info("hello", slog.String("test", "dksjdjaks"))
}
func Test_Wrapper(t *testing.T) {
	ctx := context.Background()
	slog.InfoContext(ctx, "sadasdasdasda")
	logger := GetLogger()
	logger.WithContext(ctx).Info("test", "hjhk", "sadasd")
}
