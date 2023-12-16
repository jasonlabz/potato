package slogx

import (
	"context"
	"github.com/google/uuid"
	"github.com/jasonlabz/potato/core/consts"
	"github.com/jasonlabz/potato/core/utils"
	"log/slog"
	"strings"
)

var logField = []string{consts.ContextTraceID, consts.ContextUserID, consts.ContextRemoteAddr}

func slogField(ctx context.Context, contextKey ...string) (fields []any) {
	for _, key := range contextKey {
		value := utils.GetString(ctx.Value(key))
		if value == "" && key == consts.ContextTraceID {
			value = strings.ReplaceAll(uuid.New().String(), consts.SignDash, consts.EmptyString)
		}
		if value == "" {
			continue
		}
		fields = append(fields, slog.String(key, value))
	}
	return
}

func GetLogger(ctx context.Context) *slog.Logger {
	return logger().With(slogField(ctx, logField...)...)
}

func GormLogger(ctx context.Context) *slog.Logger {
	return logger().With(slogField(ctx, logField...)...)
}
