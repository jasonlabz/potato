package rabbitmqx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newManagementTestOperator(serverURL string) *RabbitMQOperator {
	host := strings.TrimPrefix(serverURL, "http://")
	parts := strings.Split(host, ":")
	return &RabbitMQOperator{config: &MQConfig{Host: parts[0], Port: 5672, ManagementPort: intFromString(parts[1]), Username: "guest", Password: "guest"}}
}

func intFromString(s string) int {
	var n int
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

func TestMQConfigAddrUsesVirtualHost(t *testing.T) {
	addr := (&MQConfig{Host: "127.0.0.1", Port: 5672, Username: "guest", Password: "guest", VirtualHost: "tenant-a"}).addr()
	if addr != "amqp://guest:guest@127.0.0.1:5672/tenant-a" {
		t.Fatalf("addr = %q", addr)
	}
}

func TestMQConfigAddrDefaultsToRootVirtualHost(t *testing.T) {
	addr := (&MQConfig{Host: "127.0.0.1", Port: 5672, Username: "guest", Password: "guest"}).addr()
	if addr != "amqp://guest:guest@127.0.0.1:5672/%2F" {
		t.Fatalf("addr = %q", addr)
	}
}

func TestNewRabbitMQManagementOperatorDoesNotConnectAMQP(t *testing.T) {
	op := NewRabbitMQManagementOperator(&MQConfig{Host: "127.0.0.1", Port: 1, ManagementPort: 15672, Username: "guest", Password: "guest"})
	if op == nil || op.config.Host != "127.0.0.1" {
		t.Fatalf("operator = %#v", op)
	}
}

func TestGetQueueInfoUsesConfiguredVirtualHost(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.RequestURI, "/bindings") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`{"messages":0}`))
	}))
	defer server.Close()

	op := newManagementTestOperator(server.URL)
	op.config.VirtualHost = "tenant-a"
	if _, err := op.GetQueueInfo(context.Background(), "orders"); err != nil {
		t.Fatalf("get queue info: %v", err)
	}
	if !strings.HasPrefix(requestPath, "/api/queues/tenant-a/orders") {
		t.Fatalf("request path = %q", requestPath)
	}
}

func TestGetQueueInfoUsesManagementAPI(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.RequestURI)
		if user, pass, ok := r.BasicAuth(); !ok || user != "guest" || pass != "guest" {
			t.Fatalf("unexpected basic auth user=%q pass=%q ok=%v", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/queues/%2F/orders":
			_, _ = w.Write([]byte(`{"messages":12,"messages_ready":9,"messages_unacknowledged":3,"consumer_details":[{"consumer_tag":"consumer-a","channel_details":{"name":"10.0.0.1:5672 -> 10.0.0.2:53210"}}]}`))
		case "/api/queues/%2F/orders/bindings":
			_, _ = w.Write([]byte(`[{"source":"orders.exchange","routing_key":"orders.*","source_details":{"type":"topic"},"arguments":{"x-match":"all"}}]`))
		default:
			t.Fatalf("unexpected path %s", r.RequestURI)
		}
	}))
	defer server.Close()

	info, err := newManagementTestOperator(server.URL).GetQueueInfo(context.Background(), "orders")
	if err != nil {
		t.Fatalf("get queue info: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths = %#v", paths)
	}
	if info.Messages != 12 || info.MessagesReady != 9 || info.MessagesUnacknowledged != 3 {
		t.Fatalf("queue stats = %#v", info)
	}
	if len(info.Bindings) != 1 || info.Bindings[0].Exchange != "orders.exchange" || info.Bindings[0].ExchangeType != "topic" || info.Bindings[0].RoutingKey != "orders.*" {
		t.Fatalf("bindings = %#v", info.Bindings)
	}
	if len(info.Consumers) != 1 || info.Consumers[0].Name != "consumer-a" || info.Consumers[0].Channel == "" {
		t.Fatalf("consumers = %#v", info.Consumers)
	}
}

func TestPreviewQueueMessagesUsesManagementAPIWithRequeue(t *testing.T) {
	var requestPath string
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.RequestURI
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"payload":"hello","properties":{"message_id":"k1","headers":{"source":"test"}},"routing_key":"orders"}]`))
	}))
	defer server.Close()

	messages, err := newManagementTestOperator(server.URL).PreviewQueueMessages(context.Background(), "orders", 3)
	if err != nil {
		t.Fatalf("preview messages: %v", err)
	}
	if requestPath != "/api/queues/%2F/orders/get" {
		t.Fatalf("request path = %q", requestPath)
	}
	if requestBody["requeue"] != true || requestBody["ackmode"] != "ack_requeue_true" || requestBody["count"] != float64(3) {
		t.Fatalf("request body = %#v", requestBody)
	}
	if len(messages) != 1 || messages[0].Payload != "hello" || messages[0].MessageID != "k1" || messages[0].Headers["source"] != "test" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestListQueuesUsesManagementAPI(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"orders"},{"name":"payments"}]`))
	}))
	defer server.Close()

	queues, err := newManagementTestOperator(server.URL).ListQueues(context.Background())
	if err != nil {
		t.Fatalf("list queues: %v", err)
	}
	if requestPath != "/api/queues" {
		t.Fatalf("request path = %q", requestPath)
	}
	if len(queues) != 2 || queues[0] != "orders" || queues[1] != "payments" {
		t.Fatalf("queues = %#v", queues)
	}
}

func TestListExchangesSkipsDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/api/exchanges" {
			t.Fatalf("request path = %q", r.RequestURI)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":""},{"name":"amq.direct"},{"name":"orders.exchange"}]`))
	}))
	defer server.Close()

	exchanges, err := newManagementTestOperator(server.URL).ListExchanges(context.Background())
	if err != nil {
		t.Fatalf("list exchanges: %v", err)
	}
	if len(exchanges) != 1 || exchanges[0] != "orders.exchange" {
		t.Fatalf("exchanges = %#v", exchanges)
	}
}
