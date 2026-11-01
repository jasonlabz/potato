package slogx

import (
	"log/slog"
	"testing"
)

func TestReplaceAttrUserLevelField(t *testing.T) {
	// 分组内或非 slog.Level 类型的 level 字段不能 panic，原样保留
	for _, groups := range [][]string{nil, {"g"}} {
		a := replaceAttr(groups, slog.String(slog.LevelKey, "custom"))
		if a.Value.String() != "custom" {
			t.Fatalf("replaceAttr(%v) = %v, want custom", groups, a.Value)
		}
	}
	if a := replaceAttr(nil, slog.Any(slog.LevelKey, slog.LevelWarn)); a.Value.String() != "WARN" {
		t.Fatalf("replaceAttr(level) = %v, want WARN", a.Value)
	}
}

func TestWithDoesNotMutateReceiver(t *testing.T) {
	base := GetLogger()
	_ = base.WithAny("k", "v").WithCallerSkip(5)
	if base.callerSkip == 5 || base.logger.Handler() != GetLogger().logger.Handler() {
		t.Fatalf("With* mutated receiver")
	}
}
