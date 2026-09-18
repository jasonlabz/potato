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

// recordingServer 起一个按 status 返回的测试服务端。
func recordingServer(t *testing.T, status *atomic.Int32) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// recordingClient 返回一个 Client，其 logger 会把 info 及以上级别的日志消息
// 收集到 messages，用于断言 hook 的最终成败记录。
func recordingClient(t *testing.T, endpoint string, messages *[]string) *Client {
	t.Helper()

	return NewHttpClient(&Config{
		Name:     "hook-recording-test-" + t.Name(),
		Protocol: "http",
		Endpoint: endpoint,
		Timeout:  DefaultTimeout,
		logger: log.GetLogger().WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			*messages = append(*messages, entry.Message)
			return nil
		})),
	})
}

func hasLoggedMessage(messages []string, want string) bool {
	for _, message := range messages {
		if message == want {
			return true
		}
	}
	return false
}

// TestHooksReportFinalOutcome 固定 OnSuccess / OnError 的触发条件。
func TestHooksReportFinalOutcome(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantLogged string
	}{
		{name: "2xx", status: http.StatusOK, wantLogged: "httpx request succeeded"},
		// resty 的 OnSuccess 语义是「执行完成且无 error」，非 2xx 同样走成功分支，
		// 没有任何 error 会交给 OnError。这条断言就是防止语义被误读。
		{name: "5xx", status: http.StatusInternalServerError, wantLogged: "httpx request succeeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status atomic.Int32
			status.Store(int32(tt.status))
			messages := make([]string, 0, 1)
			client := recordingClient(t, recordingServer(t, &status).URL, &messages)

			var result map[string]any
			if _, err := client.Post(context.Background(), "/echo", map[string]string{"a": "b"}, &result); err != nil {
				t.Fatalf("Post() error = %v", err)
			}
			if !hasLoggedMessage(messages, tt.wantLogged) {
				t.Fatalf("logged messages = %v, want %q", messages, tt.wantLogged)
			}
		})
	}
}

// TestHooksReportTransportFailure 固定传输层失败走 OnError。
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

	messages := make([]string, 0, 1)
	client := recordingClient(t, endpoint, &messages)

	var result map[string]any
	if _, err = client.Post(context.Background(), "/echo", map[string]string{"a": "b"}, &result); err == nil {
		t.Fatal("Post() error = nil, want a transport failure")
	}
	if !hasLoggedMessage(messages, "httpx request failed") {
		t.Fatalf("logged messages = %v, want %q", messages, "httpx request failed")
	}
}
