package httpx

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jasonlabz/potato/log"
)

type contextProbeKey struct{}

// TestRequestEntriesPropagateContext 要求每个请求入口都把调用方 ctx 交给 resty。
// 少了 SetContext，上游取消不会中断在途请求与重试，req.Context() 也只会拿到
// context.Background()，导致从请求上取 trace 信息拿不到任何东西。
func TestRequestEntriesPropagateContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewHttpClient(&Config{
		Name:     "context-propagation-test",
		Protocol: "http",
		Endpoint: server.URL,
		Timeout:  DefaultTimeout,
	})

	captured := make([]context.Context, 0, 4)
	client.GetRestyClient().OnBeforeRequest(func(_ *resty.Client, request *resty.Request) error {
		captured = append(captured, request.Context())
		return nil
	})

	ctx := context.WithValue(context.Background(), contextProbeKey{}, "trace-123")
	var result map[string]any
	if err := client.Get(ctx, "/echo", &result); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := client.Post(ctx, "/echo", map[string]string{"a": "b"}, &result); err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if _, err := client.PostForm(ctx, "/echo", map[string]string{"a": "b"}, &result); err != nil {
		t.Fatalf("PostForm() error = %v", err)
	}
	if _, err := client.PostMultipart(ctx, "/echo", nil, map[string]string{"a": "b"}, &result); err != nil {
		t.Fatalf("PostMultipart() error = %v", err)
	}

	if len(captured) != 4 {
		t.Fatalf("captured request count = %d, want 4", len(captured))
	}
	for index, requestCtx := range captured {
		if got := requestCtx.Value(contextProbeKey{}); got != "trace-123" {
			t.Errorf("request %d context value = %v, want trace-123", index, got)
		}
	}
}

// recordingServer 起一个按 status 返回的测试服务端；失败时返回可识别的错误体。
func recordingServer(t *testing.T, status *atomic.Int32) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		code := int(status.Load())
		w.WriteHeader(code)
		if code >= http.StatusBadRequest {
			_, _ = w.Write([]byte(failureBody))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server
}

const (
	successMessage = "[rpc] HTTP Request  succeeded"
	failureMessage = "[rpc] HTTP Request  failed"
	failureBody    = `{"error":"boom"}`
)

// recordedLog 是一条被捕获的日志。
type recordedLog struct {
	message string
	fields  map[string]string
}

// recordingCore 把日志的消息与字段收集下来，便于断言 hook 打出的内容。
type recordingCore struct {
	zapcore.Core
	logs *[]recordedLog
}

func (c *recordingCore) With(fields []zapcore.Field) zapcore.Core {
	// hook 里的 WithError(err) 会走这里，保持同一个收集切片。
	return &recordingCore{Core: c.Core.With(fields), logs: c.logs}
}

func (c *recordingCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Enabled(entry.Level) {
		return checked
	}
	return checked.AddCore(entry, c)
}

func (c *recordingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	collected := make(map[string]string, len(fields))
	for _, field := range fields {
		collected[field.Key] = field.String
	}
	*c.logs = append(*c.logs, recordedLog{message: entry.Message, fields: collected})
	return c.Core.Write(entry, fields)
}

// recordingClient 返回一个 Client，其 logger 会把 info 及以上级别的日志收集到 logs。
func recordingClient(t *testing.T, endpoint string, logs *[]recordedLog) *Client {
	t.Helper()

	return NewHttpClient(&Config{
		Name:     "hook-recording-test-" + t.Name(),
		Protocol: "http",
		Endpoint: endpoint,
		Timeout:  DefaultTimeout,
		logger: log.GetLogger().WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return &recordingCore{Core: core, logs: logs}
		})),
	})
}

func findLogged(logs []recordedLog, message string) (recordedLog, bool) {
	for _, entry := range logs {
		if entry.message == message {
			return entry, true
		}
	}
	return recordedLog{}, false
}

// TestHooksReportFinalOutcome 固定成败判定：resty 的 OnSuccess 语义是「执行无 error」，
// 4xx/5xx 也会落在 OnSuccess 里，wrapper 在这一层把它们记为失败并带上响应体。
func TestHooksReportFinalOutcome(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		wantMessage string
	}{
		{name: "2xx", status: http.StatusOK, wantMessage: successMessage},
		{name: "5xx", status: http.StatusInternalServerError, wantMessage: failureMessage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status atomic.Int32
			status.Store(int32(tt.status))
			logs := make([]recordedLog, 0, 1)
			client := recordingClient(t, recordingServer(t, &status).URL, &logs)

			var result map[string]any
			if _, err := client.Post(context.Background(), "/echo", map[string]string{"a": "b"}, &result); err != nil {
				t.Fatalf("Post() error = %v", err)
			}
			entry, ok := findLogged(logs, tt.wantMessage)
			if !ok {
				t.Fatalf("logged entries = %v, want message %q", logs, tt.wantMessage)
			}
			if tt.status < http.StatusBadRequest {
				return
			}
			if got := entry.fields["body"]; got != failureBody {
				t.Fatalf("failure body field = %q, want %q", got, failureBody)
			}
		})
	}
}

// TestHooksReportTransportFailure 固定传输层失败走 OnError（此时没有响应体可打）。
func TestHooksReportTransportFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	// 立刻释放端口，制造确定的连接失败；Config.RetryCount 为 0，不会重试。
	endpoint := "http://" + listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	logs := make([]recordedLog, 0, 1)
	client := recordingClient(t, endpoint, &logs)

	var result map[string]any
	if _, err = client.Post(context.Background(), "/echo", map[string]string{"a": "b"}, &result); err == nil {
		t.Fatal("Post() error = nil, want a transport failure")
	}
	entry, ok := findLogged(logs, failureMessage)
	if !ok {
		t.Fatalf("logged entries = %v, want message %q", logs, failureMessage)
	}
	if _, ok = entry.fields["body"]; ok {
		t.Fatalf("transport failure must not fabricate a body, got %q", entry.fields["body"])
	}
}

// TestHooksReportDecodeFailureWithBody 覆盖「响应已拿到但解析失败」这条失败路径：
// resty 会把响应包进 *ResponseError，要求把上游原始响应体一起打出来。
func TestHooksReportDecodeFailureWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	logs := make([]recordedLog, 0, 1)
	client := recordingClient(t, server.URL, &logs)

	var result map[string]any
	if _, err := client.Post(context.Background(), "/echo", map[string]string{"a": "b"}, &result); err == nil {
		t.Fatal("Post() error = nil, want a decode failure")
	}
	entry, ok := findLogged(logs, failureMessage)
	if !ok {
		t.Fatalf("logged entries = %v, want message %q", logs, failureMessage)
	}
	if got := entry.fields["body"]; got != "not-json" {
		t.Fatalf("failure body field = %q, want %q", got, "not-json")
	}
}
