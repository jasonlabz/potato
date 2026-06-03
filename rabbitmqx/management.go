package rabbitmqx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

type QueueInfo struct {
	Messages               int64           `json:"messages"`
	MessagesReady          int64           `json:"messages_ready"`
	MessagesUnacknowledged int64           `json:"messages_unacknowledged"`
	Bindings               []QueueBinding  `json:"bindings"`
	Consumers              []QueueConsumer `json:"consumers"`
}

type QueueBinding struct {
	Exchange     string         `json:"exchange"`
	ExchangeType string         `json:"exchange_type,omitempty"`
	RoutingKey   string         `json:"routing_key,omitempty"`
	Arguments    map[string]any `json:"arguments,omitempty"`
}

type QueueConsumer struct {
	Name    string `json:"name"`
	Channel string `json:"channel,omitempty"`
}

type QueueMessage struct {
	Payload    string            `json:"payload"`
	PayloadRaw string            `json:"payload_raw,omitempty"`
	MessageID  string            `json:"message_id,omitempty"`
	RoutingKey string            `json:"routing_key,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

func NewRabbitMQManagementOperator(config *MQConfig) *RabbitMQOperator {
	return &RabbitMQOperator{config: config}
}

func (r *RabbitMQOperator) managementPort() int {
	if r.config.ManagementPort > 0 {
		return r.config.ManagementPort
	}
	return 15672
}

func (r *RabbitMQOperator) managementURL(path string) string {
	return fmt.Sprintf("http://%s:%d/api/%s", r.config.Host, r.managementPort(), path)
}

func (r *RabbitMQOperator) virtualHost() string {
	if r.config.VirtualHost != "" {
		return r.config.VirtualHost
	}
	return "/"
}

func (r *RabbitMQOperator) queuePath(queue string, suffix string) string {
	path := fmt.Sprintf("queues/%s/%s", url.PathEscape(r.virtualHost()), url.PathEscape(queue))
	if suffix != "" {
		path += "/" + suffix
	}
	return path
}

func (r *RabbitMQOperator) managementGet(ctx context.Context, path string) ([]byte, error) {
	return r.managementRequest(ctx, http.MethodGet, path, nil)
}

func (r *RabbitMQOperator) managementPost(ctx context.Context, path string, payload any) ([]byte, error) {
	body, err := sonic.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: marshal management request: %w", err)
	}
	return r.managementRequest(ctx, http.MethodPost, path, body)
}

func (r *RabbitMQOperator) managementRequest(ctx context.Context, method string, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, r.managementURL(path), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: create management request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(r.config.Username, r.config.Password)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: management API %s: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: read management response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rabbitmq: management API %s returned %d: %s", req.URL.String(), resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (r *RabbitMQOperator) ListQueues(ctx context.Context) ([]string, error) {
	body, err := r.managementGet(ctx, "queues")
	if err != nil {
		return nil, err
	}
	var queues []struct {
		Name string `json:"name"`
	}
	if err := sonic.Unmarshal(body, &queues); err != nil {
		return nil, fmt.Errorf("rabbitmq: parse queues: %w", err)
	}
	names := make([]string, 0, len(queues))
	for _, queue := range queues {
		names = append(names, queue.Name)
	}
	return names, nil
}

func (r *RabbitMQOperator) ListExchanges(ctx context.Context) ([]string, error) {
	body, err := r.managementGet(ctx, "exchanges")
	if err != nil {
		return nil, err
	}
	var exchanges []struct {
		Name string `json:"name"`
	}
	if err := sonic.Unmarshal(body, &exchanges); err != nil {
		return nil, fmt.Errorf("rabbitmq: parse exchanges: %w", err)
	}
	names := make([]string, 0, len(exchanges))
	for _, exchange := range exchanges {
		if exchange.Name == "" || strings.HasPrefix(exchange.Name, "amq.") {
			continue
		}
		names = append(names, exchange.Name)
	}
	return names, nil
}

func (r *RabbitMQOperator) GetQueueInfo(ctx context.Context, queue string) (*QueueInfo, error) {
	body, err := r.managementGet(ctx, r.queuePath(queue, ""))
	if err != nil {
		return nil, err
	}
	var queueStats struct {
		Messages               int64 `json:"messages"`
		MessagesReady          int64 `json:"messages_ready"`
		MessagesUnacknowledged int64 `json:"messages_unacknowledged"`
		ConsumerDetails        []struct {
			ConsumerTag string `json:"consumer_tag"`
			Channel     struct {
				Name string `json:"name"`
			} `json:"channel_details"`
		} `json:"consumer_details"`
	}
	if err := sonic.Unmarshal(body, &queueStats); err != nil {
		return nil, fmt.Errorf("rabbitmq: parse queue stats: %w", err)
	}

	var rawBindings []struct {
		Source        string         `json:"source"`
		RoutingKey    string         `json:"routing_key"`
		Arguments     map[string]any `json:"arguments"`
		SourceDetails map[string]any `json:"source_details"`
	}
	bindings := []QueueBinding{}
	if body, err = r.managementGet(ctx, r.queuePath(queue, "bindings")); err == nil {
		if err := sonic.Unmarshal(body, &rawBindings); err != nil {
			return nil, fmt.Errorf("rabbitmq: parse queue bindings: %w", err)
		}
		bindings = make([]QueueBinding, 0, len(rawBindings))
		for _, rb := range rawBindings {
			binding := QueueBinding{Exchange: rb.Source, RoutingKey: rb.RoutingKey, Arguments: rb.Arguments}
			if typ, ok := rb.SourceDetails["type"].(string); ok {
				binding.ExchangeType = typ
			}
			bindings = append(bindings, binding)
		}
	}

	consumers := make([]QueueConsumer, 0, len(queueStats.ConsumerDetails))
	for _, rc := range queueStats.ConsumerDetails {
		consumers = append(consumers, QueueConsumer{Name: rc.ConsumerTag, Channel: rc.Channel.Name})
	}
	return &QueueInfo{
		Messages:               queueStats.Messages,
		MessagesReady:          queueStats.MessagesReady,
		MessagesUnacknowledged: queueStats.MessagesUnacknowledged,
		Bindings:               bindings,
		Consumers:              consumers,
	}, nil
}

func (r *RabbitMQOperator) PreviewQueueMessages(ctx context.Context, queue string, limit int) ([]QueueMessage, error) {
	if limit <= 0 {
		limit = 10
	}
	body, err := r.managementPost(ctx, r.queuePath(queue, "get"), map[string]any{
		"count":    limit,
		"ackmode":  "ack_requeue_true",
		"encoding": "auto",
		"requeue":  true,
	})
	if err != nil {
		return nil, err
	}
	var items []struct {
		Payload    string         `json:"payload"`
		PayloadRaw string         `json:"payload_raw"`
		RoutingKey string         `json:"routing_key"`
		Properties map[string]any `json:"properties"`
	}
	if err := sonic.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("rabbitmq: parse preview messages: %w", err)
	}
	messages := make([]QueueMessage, 0, len(items))
	for _, item := range items {
		message := QueueMessage{Payload: item.Payload, PayloadRaw: item.PayloadRaw, RoutingKey: item.RoutingKey, Headers: map[string]string{}}
		if id, ok := item.Properties["message_id"].(string); ok {
			message.MessageID = id
		}
		if headers, ok := item.Properties["headers"].(map[string]any); ok {
			for k, v := range headers {
				message.Headers[k] = fmt.Sprintf("%v", v)
			}
		}
		messages = append(messages, message)
	}
	return messages, nil
}
