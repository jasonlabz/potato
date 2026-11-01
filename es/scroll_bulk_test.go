package es

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeES 模拟 scroll 两页加一页空结果、clear scroll 与 bulk 部分失败。
func fakeES(cleared *atomic.Int32) *httptest.Server {
	var page atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/_search/scroll"):
			cleared.Add(1)
			_, _ = w.Write([]byte(`{"succeeded":true}`))
		case strings.HasSuffix(r.URL.Path, "/_search") || strings.HasPrefix(r.URL.Path, "/_search/scroll"):
			hits := []string{`[{"_id":"1","_source":{"n":1}},{"_id":"2","_source":{"n":2}}]`, `[{"_id":"3","_source":{"n":3}}]`, `[]`}
			_, _ = w.Write([]byte(`{"_scroll_id":"sid","hits":{"hits":` + hits[page.Add(1)-1] + `}}`))
		case r.URL.Path == "/_bulk":
			_, _ = w.Write([]byte(`{"errors":true,"items":[{"index":{"_id":"1","status":201}},` +
				`{"index":{"_id":"2","status":400,"error":{"type":"mapper_parsing_exception"}}}]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

func TestScrollEachPagesAndClears(t *testing.T) {
	var cleared atomic.Int32
	server := fakeES(&cleared)
	defer server.Close()
	endpoints := []string{strings.TrimPrefix(server.URL, "http://")}
	op, err := NewElasticSearchOperator(context.Background(), &Config{Endpoints: endpoints, CACert: []byte{}})
	if err != nil {
		t.Fatalf("NewElasticSearchOperator() error = %v", err)
	}
	if endpoints[0] != strings.TrimPrefix(server.URL, "http://") {
		t.Fatalf("config.Endpoints mutated to %q", endpoints[0])
	}

	var batches []int
	err = op.ScrollEach(context.Background(), "idx", nil, time.Minute, 2, func(hits []map[string]any) error {
		batches = append(batches, len(hits))
		return nil
	})
	if err != nil || len(batches) != 2 || batches[0] != 2 || batches[1] != 1 || cleared.Load() != 1 {
		t.Fatalf("ScrollEach() batches = %v, cleared = %d, err = %v", batches, cleared.Load(), err)
	}
}

func TestBulkReportsItemErrors(t *testing.T) {
	var cleared atomic.Int32
	server := fakeES(&cleared)
	defer server.Close()
	op, err := NewElasticSearchOperator(context.Background(), &Config{Endpoints: []string{server.URL}})
	if err != nil {
		t.Fatalf("NewElasticSearchOperator() error = %v", err)
	}

	out, err := op.Bulk(context.Background(), []BulkOperation{
		{Action: "index", Index: "idx", DocID: "1", Doc: map[string]any{"n": 1}},
		{Action: "index", Index: "idx", DocID: "2", Doc: map[string]any{"n": "x"}},
	})
	if err == nil || !strings.Contains(err.Error(), "_id=2") || out == nil {
		t.Fatalf("Bulk() out = %v, err = %v, want item error for _id=2", out, err)
	}
}
