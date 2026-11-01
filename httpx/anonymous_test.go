package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/jasonlabz/potato/log"
)

// TestAnonymousClientsAreIndependent 固定按调用方创建客户端的用法：Name 为空不登记重名，
// 每个实例的认证互不影响。
func TestAnonymousClientsAreIndependent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	defer server.Close()

	for _, token := range []string{"a", "b"} {
		client := NewHttpClient(&Config{Protocol: "http", Endpoint: server.URL, Timeout: DefaultTimeout})
		client.SetGlobalToken(token)
		resp, err := client.Do(context.Background(), http.MethodGet, "/", nil, nil)
		if err != nil || resp.String() != "Bearer "+token {
			t.Fatalf("Do() = %q, %v, want Bearer %s", resp.String(), err, token)
		}
		_ = client.Close()
	}
}

func TestDoReturnsStatusForAnyMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(r.Method))
	}))
	defer server.Close()
	client := NewHttpClient(&Config{Protocol: "http", Endpoint: server.URL, Timeout: DefaultTimeout})

	for _, method := range []string{http.MethodGet, "post", "MKCOL"} {
		resp, err := client.Do(context.Background(), method, "/", nil, nil)
		if err != nil || !resp.IsError() || resp.String() != strings.ToUpper(method) {
			t.Fatalf("Do(%s) = %d %q, %v", method, resp.StatusCode(), resp.String(), err)
		}
	}
}

// TestQuietLogPolicy 固定两个日志开关：成功日志记 Debug，失败仍为 Error，全程不记录 body。
func TestQuietLogPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_, _ = w.Write([]byte("response-secret"))
	}))
	defer server.Close()
	core, observed := observer.New(zapcore.DebugLevel)
	client := NewHttpClient(&Config{
		Protocol: "http", Endpoint: server.URL, Timeout: DefaultTimeout,
		DisableBodyLog: true, DebugSuccessLog: true,
		logger: log.GetLogger().WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core { return core })),
	})

	_, _ = client.Post(context.Background(), "/ok", "request-secret", nil)
	_, _ = client.PostForm(context.Background(), "/fail", map[string]string{"password": "form-secret"}, nil)

	var levels []zapcore.Level
	for _, entry := range observed.All() {
		levels = append(levels, entry.Level)
		for key, value := range entry.ContextMap() {
			if text, ok := value.(string); ok && strings.Contains(text, "secret") {
				t.Fatalf("%q field %s leaks %q", entry.Message, key, text)
			}
		}
	}
	want := []zapcore.Level{
		zapcore.DebugLevel, zapcore.DebugLevel, zapcore.DebugLevel, // /ok：请求、响应、成功
		zapcore.DebugLevel, zapcore.DebugLevel, zapcore.ErrorLevel, // /fail：请求、响应、失败
	}
	if len(levels) != len(want) {
		t.Fatalf("levels = %v, want %v", levels, want)
	}
	for i := range want {
		if levels[i] != want[i] {
			t.Fatalf("levels = %v, want %v", levels, want)
		}
	}
}
