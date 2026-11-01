package obsx

import (
	"context"
	"io"
	"strings"
	"testing"
)

// fakeClient 只实现测试用到的 ListObjects、GetObject，其余方法来自嵌入的 nil 接口。
type fakeClient struct {
	Client
	keys []string
}

func (f *fakeClient) ListObjects(_ context.Context, _ string, _ ListOptions) <-chan ObjectInfo {
	ch := make(chan ObjectInfo, len(f.keys))
	for _, k := range f.keys {
		ch <- ObjectInfo{Key: k}
	}
	close(ch)
	return ch
}

func (f *fakeClient) GetObject(_ context.Context, _, key string) (io.ReadCloser, *ObjectInfo, error) {
	return io.NopCloser(strings.NewReader(key)), &ObjectInfo{Key: key}, nil
}

func TestNewMinioClientStripsScheme(t *testing.T) {
	for endpoint, want := range map[string]string{
		"https://obs.example.com:9000": "https",
		"http://obs.example.com:9000":  "http",
		"obs.example.com:9000":         "http",
		"127.0.0.1:9000":               "http",
	} {
		c, err := newMinioClient(&Config{Endpoint: endpoint, AccessKey: "ak", SecretKey: "sk", Timeout: 1})
		if err != nil {
			t.Fatalf("newMinioClient(%q) error = %v", endpoint, err)
		}
		if got := c.client.EndpointURL(); got.Scheme != want || got.Host != strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://") {
			t.Fatalf("newMinioClient(%q) endpoint = %v, want scheme %s", endpoint, got, want)
		}
	}
}

func TestGetClientEmptyNameReturnsFirstRegistered(t *testing.T) {
	op, err := NewOperator(&Config{Name: "first", Endpoint: "a:9000", AccessKey: "ak", SecretKey: "sk"})
	if err != nil {
		t.Fatalf("NewOperator() error = %v", err)
	}
	for _, name := range []string{"b", "c", "d", "e"} {
		if err := op.RegisterClient(&Config{Name: name, Endpoint: name + ":9000", AccessKey: "ak", SecretKey: "sk"}); err != nil {
			t.Fatalf("RegisterClient(%s) error = %v", name, err)
		}
	}
	want, _ := op.GetClient("first")
	for i := 0; i < 20; i++ {
		if got, _ := op.GetClient(""); got != want {
			t.Fatalf("GetClient(\"\") returned a non-default client")
		}
	}
}

func TestFGetDirectoryRejectsPathTraversal(t *testing.T) {
	op := &Operator{clients: map[string]Client{"f": &fakeClient{keys: []string{"p/../../evil"}}}}
	err := op.FGetDirectory(context.Background(), "f", "bucket", "p", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("FGetDirectory() err = %v, want path traversal error", err)
	}
}
