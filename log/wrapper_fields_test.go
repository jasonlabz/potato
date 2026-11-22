package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newBufferLogger(buf *bytes.Buffer) *LoggerWrapper {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(buf),
		zapcore.DebugLevel,
	)
	return &LoggerWrapper{logger: zap.New(core)}
}

func decodeLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("没有日志输出")
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(line), &out); err != nil {
		t.Fatalf("解析日志行失败 %q: %v", line, err)
	}
	return out
}

// TestCheckFields 覆盖字段归一化的各种写法，重点保证不丢字段、不丢类型。
func TestCheckFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newBufferLogger(buf)
	ctx := context.Background()

	cases := []struct {
		name   string
		fields []any
		want   map[string]any
	}{
		{
			name:   "全部是 zap.Field",
			fields: []any{String("a", "1"), Int("b", 2)},
			want:   map[string]any{"a": "1", "b": float64(2)},
		},
		{
			name:   "全部是键值对",
			fields: []any{"a", "1", "b", 2},
			want:   map[string]any{"a": "1", "b": float64(2)},
		},
		{
			name:   "zap.Field 在前、键值对在后",
			fields: []any{String("a", "1"), "b", "2", "c", "3"},
			want:   map[string]any{"a": "1", "b": "2", "c": "3"},
		},
		{
			name:   "键值对在前、zap.Field 在后",
			fields: []any{"a", "1", String("b", "2")},
			want:   map[string]any{"a": "1", "b": "2"},
		},
		{
			name:   "保留原始类型",
			fields: []any{"count", 42, "ok", true, "ratio", 0.5},
			want:   map[string]any{"count": float64(42), "ok": true, "ratio": 0.5},
		},
		{
			name:   "error 作为值",
			fields: []any{"err", errors.New("boom")},
			want:   map[string]any{"err": "boom"},
		},
		{
			name:   "单独的 error",
			fields: []any{errors.New("boom")},
			want:   map[string]any{"error": "boom"},
		},
		{
			name:   "nil 值",
			fields: []any{"k", nil},
			want:   map[string]any{"k": nil},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf.Reset()
			l.Info(ctx, "msg", c.fields...)
			got := decodeLine(t, buf)
			for k, want := range c.want {
				v, ok := got[k]
				if !ok {
					t.Fatalf("缺少字段 %q，实际输出 %v", k, got)
				}
				if v != want {
					t.Fatalf("字段 %q = %#v，期望 %#v", k, v, want)
				}
			}
			if len(got) != len(c.want)+3 { // level、ts、msg
				t.Fatalf("字段数量不符，实际输出 %v", got)
			}
		})
	}
}

// TestCheckFieldsBadCall 保证异常调用不会静默吞掉字段。
func TestCheckFieldsBadCall(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newBufferLogger(buf)
	ctx := context.Background()

	t.Run("奇数个参数", func(t *testing.T) {
		buf.Reset()
		l.Info(ctx, "msg", "a", "1", "orphan")
		got := decodeLine(t, buf)
		if got["a"] != "1" {
			t.Fatalf("配对的字段丢失: %v", got)
		}
		if got[badKey] != "orphan" {
			t.Fatalf("孤立参数未标记为 %s: %v", badKey, got)
		}
	})

	t.Run("非字符串 key", func(t *testing.T) {
		buf.Reset()
		l.Info(ctx, "msg", 123, "v")
		line := buf.String()
		if !strings.Contains(line, badKey) {
			t.Fatalf("非法 key 未被标记: %s", line)
		}
	})
}

// TestContextFields 校验 ctx 中的字段注入，以及空值跳过、nil ctx 不 panic。
func TestContextFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newBufferLogger(buf)
	l.logField = []string{"trace_id", "user_id"}

	buf.Reset()
	l.Info(context.WithValue(context.Background(), "trace_id", "t-1"), "msg")
	if got := decodeLine(t, buf); got["trace_id"] != "t-1" {
		t.Fatalf("ctx 字段未注入: %v", got)
	}

	buf.Reset()
	l.Info(context.Background(), "msg")
	if got := decodeLine(t, buf); got["trace_id"] != nil {
		t.Fatalf("空值不应输出: %v", got)
	}

	buf.Reset()
	l.Info(nil, "msg") //nolint:staticcheck // 显式验证 nil ctx
	if got := decodeLine(t, buf); got["msg"] != "msg" {
		t.Fatalf("nil ctx 日志异常: %v", got)
	}
}

func TestSetLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	custom := newBufferLogger(buf)
	prev := defaultLogger.Swap(custom)
	t.Cleanup(func() { defaultLogger.Store(prev) })

	if got := GetLogger(); got != custom {
		t.Fatal("SetLogger 后 GetLogger 未返回注入的实例")
	}
	if got := GetLogger().WithOptions(); got == custom {
		t.Fatal("WithOptions 应返回副本")
	}
}
