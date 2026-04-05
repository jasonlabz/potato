package kafkax

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/utils"
)

var operator *KafkaOperator

// GetKafkaOperator 获取全局 Kafka 操作器实例
func GetKafkaOperator() *KafkaOperator {
	return operator
}

func init() {
	appConf := configx.GetConfig()
	if appConf.Kafka.Enable {
		mqConf := &MQConfig{}
		err := utils.CopyStruct(appConf.Kafka, mqConf)
		if err != nil {
			zapx.GetLogger().WithError(err).Error(context.Background(), "copy kafka config error, skipping ...")
			return
		}
		operator, err = NewKafkaOperator(mqConf)
		if err == nil {
			return
		}
		zapx.GetLogger().WithError(err).Error(context.Background(), "init kafka Client error")
		if appConf.Kafka.Strict {
			panic(fmt.Errorf("init kafka Client error: %v", err))
		}
	}
}

const (
	DefaultTimeOut             = 10 * time.Second
	DefaultRetryWaitTime       = 2 * time.Second
	DefaultRetryTimes          = 3
	Closed               int32 = 1
)

// MQConfig Kafka 连接配置
type MQConfig struct {
	BootstrapServers []string `json:"bootstrap_servers"`
	GroupID          string   `json:"group_id"`
	Topics           []string `json:"topic"`
	SecurityProtocol string   `json:"security_protocol"`
	SaslMechanism    string   `json:"sasl_mechanism"`
	SaslUsername     string   `json:"sasl_username"`
	SaslPassword     string   `json:"sasl_password"`
	MaxAttempts      int      `json:"max_attempts"`
	RetryWaitTime    int64    `json:"retry_wait_time"` // 秒
}

// Validate 验证配置参数
func (c *MQConfig) Validate() error {
	if len(c.BootstrapServers) == 0 {
		return errors.New("bootstrap_servers is empty")
	}
	if c.MaxAttempts == 0 {
		c.MaxAttempts = DefaultRetryTimes
	}
	if c.RetryWaitTime == 0 {
		c.RetryWaitTime = int64(DefaultRetryWaitTime / time.Second)
	}
	return nil
}

// getSASLMechanism 获取 SASL 认证机制
func (c *MQConfig) getSASLMechanism() (sasl.Mechanism, error) {
	switch c.SaslMechanism {
	case "PLAIN":
		return &plain.Mechanism{
			Username: c.SaslUsername,
			Password: c.SaslPassword,
		}, nil
	case "SCRAM-SHA-256":
		return scram.Mechanism(scram.SHA256, c.SaslUsername, c.SaslPassword)
	case "SCRAM-SHA-512":
		return scram.Mechanism(scram.SHA512, c.SaslUsername, c.SaslPassword)
	default:
		return nil, nil
	}
}

// KafkaOperator Kafka 操作器
type KafkaOperator struct {
	name    string
	config  *MQConfig
	connCfg *ConnConfig

	writers    sync.Map // topic -> *kafka.Writer
	writersMu  sync.Mutex
	readers    sync.Map // topic+groupID -> *kafka.Reader
	readersMu  sync.Mutex
	cancelChan sync.Map // topic+groupID -> chan bool

	transport *kafka.Transport
	closeCh   chan bool
	closed    int32
	mu        sync.Mutex
	l         log.Logger
}

// NewKafkaOperator 创建 Kafka 操作器实例
func NewKafkaOperator(config *MQConfig, opts ...ConnOption) (op *KafkaOperator, err error) {
	if err = config.Validate(); err != nil {
		return
	}

	connCfg := DefaultConfig()
	for _, opt := range opts {
		opt(connCfg)
	}

	op = &KafkaOperator{
		name:    uuid.NewString(),
		config:  config,
		connCfg: connCfg,
		closeCh: make(chan bool),
		l:       connCfg.l,
	}

	// 构建 transport
	op.transport, err = op.buildTransport()
	if err != nil {
		return nil, fmt.Errorf("build kafka transport failed: %w", err)
	}

	ctx := context.Background()
	// 验证连接
	if err = op.ping(ctx); err != nil {
		return nil, fmt.Errorf("kafka connection ping failed: %w", err)
	}

	op.l.Info(ctx, "kafka operator created successfully", "brokers", config.BootstrapServers)
	return
}

// buildTransport 构建 Kafka Transport
func (r *KafkaOperator) buildTransport() (*kafka.Transport, error) {
	t := &kafka.Transport{}

	mechanism, err := r.config.getSASLMechanism()
	if err != nil {
		return nil, fmt.Errorf("create SASL mechanism failed: %w", err)
	}
	if mechanism != nil {
		t.SASL = mechanism
	}

	if r.config.SecurityProtocol == "SSL" || r.config.SecurityProtocol == "SASL_SSL" {
		t.TLS = &tls.Config{InsecureSkipVerify: true}
	}

	return t, nil
}

// ping 验证 Kafka 连接
func (r *KafkaOperator) ping(ctx context.Context) error {
	dialer := &kafka.Dialer{
		Timeout: DefaultTimeOut,
	}

	mechanism, _ := r.config.getSASLMechanism()
	if mechanism != nil {
		dialer.SASLMechanism = mechanism
	}
	if r.config.SecurityProtocol == "SSL" || r.config.SecurityProtocol == "SASL_SSL" {
		dialer.TLS = &tls.Config{InsecureSkipVerify: true}
	}

	conn, err := dialer.DialContext(ctx, "tcp", r.config.BootstrapServers[0])
	if err != nil {
		return fmt.Errorf("dial kafka broker failed: %w", err)
	}
	_ = conn.Close()
	return nil
}

// getWriter 获取或创建指定 topic 的 Writer
func (r *KafkaOperator) getWriter(topic string) *kafka.Writer {
	if w, ok := r.writers.Load(topic); ok {
		return w.(*kafka.Writer)
	}

	r.writersMu.Lock()
	defer r.writersMu.Unlock()

	// 双重检查
	if w, ok := r.writers.Load(topic); ok {
		return w.(*kafka.Writer)
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(r.config.BootstrapServers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		MaxAttempts:  r.connCfg.MaxAttempts,
		BatchSize:    r.connCfg.BatchSize,
		BatchTimeout: r.connCfg.BatchTimeout,
		WriteTimeout: r.connCfg.WriteTimeout,
		ReadTimeout:  r.connCfg.ReadTimeout,
		Transport:    r.transport,
	}

	r.writers.Store(topic, w)
	return w
}

// getReader 获取或创建 Reader
func (r *KafkaOperator) getReader(config *ConsumeConfig) *kafka.Reader {
	key := config.Topic + ":" + config.GroupID
	if rd, ok := r.readers.Load(key); ok {
		return rd.(*kafka.Reader)
	}

	r.readersMu.Lock()
	defer r.readersMu.Unlock()

	if rd, ok := r.readers.Load(key); ok {
		return rd.(*kafka.Reader)
	}

	readerCfg := kafka.ReaderConfig{
		Brokers: r.config.BootstrapServers,
		Topic:   config.Topic,
	}

	if config.GroupID != "" {
		readerCfg.GroupID = config.GroupID
	} else if r.config.GroupID != "" {
		readerCfg.GroupID = r.config.GroupID
	}

	if config.MinBytes > 0 {
		readerCfg.MinBytes = config.MinBytes
	} else {
		readerCfg.MinBytes = r.connCfg.MinBytes
	}

	if config.MaxBytes > 0 {
		readerCfg.MaxBytes = config.MaxBytes
	} else {
		readerCfg.MaxBytes = r.connCfg.MaxBytes
	}

	if config.MaxWait > 0 {
		readerCfg.MaxWait = config.MaxWait
	}

	if config.CommitInterval > 0 {
		readerCfg.CommitInterval = config.CommitInterval
	} else {
		readerCfg.CommitInterval = r.connCfg.CommitInterval
	}

	if config.StartOffset != 0 {
		readerCfg.StartOffset = config.StartOffset
	}

	if config.Partition > 0 && config.GroupID == "" {
		readerCfg.Partition = config.Partition
	}

	dialer := &kafka.Dialer{Timeout: DefaultTimeOut}
	mechanism, _ := r.config.getSASLMechanism()
	if mechanism != nil {
		dialer.SASLMechanism = mechanism
	}
	if r.config.SecurityProtocol == "SSL" || r.config.SecurityProtocol == "SASL_SSL" {
		dialer.TLS = &tls.Config{InsecureSkipVerify: true}
	}
	readerCfg.Dialer = dialer

	rd := kafka.NewReader(readerCfg)
	r.readers.Store(key, rd)
	return rd
}

// Push 推送单条消息
func (r *KafkaOperator) Push(ctx context.Context, msg *ProduceMessage) error {
	return r.PushBatch(ctx, msg.Topic, []*ProduceMessage{msg})
}

// PushBatch 批量推送消息
func (r *KafkaOperator) PushBatch(ctx context.Context, topic string, messages []*ProduceMessage) error {
	defer handlePanic(ctx, r)

	if atomic.LoadInt32(&r.closed) == Closed {
		return errors.New("kafka operator is closed")
	}

	if topic == "" && len(messages) > 0 {
		topic = messages[0].Topic
	}
	if topic == "" {
		return errors.New("topic is required")
	}

	w := r.getWriter(topic)
	kafkaMessages := make([]kafka.Message, 0, len(messages))
	for _, m := range messages {
		kafkaMessages = append(kafkaMessages, m.toKafkaMessage())
	}

	// 带重试的推送
	var err error
	retryWait := time.Duration(r.config.RetryWaitTime) * time.Second
	for i := 0; i < r.config.MaxAttempts; i++ {
		if atomic.LoadInt32(&r.closed) == Closed {
			return errors.New("kafka operator is closed")
		}

		err = w.WriteMessages(ctx, kafkaMessages...)
		if err == nil {
			r.l.Info(ctx, "[push] kafka push success", "topic", topic, "count", len(messages))
			return nil
		}

		r.l.Warn(ctx, "[push] kafka push failed, retrying...",
			"err", err.Error(), "topic", topic, "retry_count", i+1)

		select {
		case <-r.closeCh:
			return errors.New("kafka operator closed during push")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryWait):
		}
	}

	r.l.Error(ctx, "[push] kafka push failed after retries",
		"err", err.Error(), "topic", topic, "attempts", r.config.MaxAttempts)
	return fmt.Errorf("push failed after %d retries: %w", r.config.MaxAttempts, err)
}

// Consume 消费消息，返回消息通道
func (r *KafkaOperator) Consume(ctx context.Context, config *ConsumeConfig) (<-chan kafka.Message, error) {
	if atomic.LoadInt32(&r.closed) == Closed {
		return nil, errors.New("kafka operator is closed")
	}

	if config.Topic == "" {
		return nil, errors.New("topic is required")
	}

	reader := r.getReader(config)
	key := config.Topic + ":" + config.GroupID
	contents := make(chan kafka.Message, 10)

	// 创建取消通道
	cancelCh := make(chan bool)
	r.cancelChan.Store(key, cancelCh)

	go func() {
		ctxBack := context.Background()
		defer handlePanic(ctxBack, r)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sig)

		retryWait := time.Duration(r.config.RetryWaitTime) * time.Second

		for {
			select {
			case s := <-sig:
				r.l.Info(ctxBack, fmt.Sprintf("[consume:%s] received signal: %v, exiting...", config.Topic, s))
				close(contents)
				return
			case <-r.closeCh:
				r.l.Info(ctxBack, fmt.Sprintf("[consume:%s] kafka is closed, exiting...", config.Topic))
				close(contents)
				return
			case <-cancelCh:
				r.l.Info(ctxBack, fmt.Sprintf("[consume:%s] consume canceled, exiting...", config.Topic))
				r.cancelChan.Delete(key)
				close(contents)
				return
			default:
			}

			msg, err := reader.ReadMessage(ctxBack)
			if err != nil {
				if atomic.LoadInt32(&r.closed) == Closed {
					close(contents)
					return
				}
				r.l.Error(ctxBack, fmt.Sprintf("[consume:%s] read message error, retrying...", config.Topic),
					"err", err.Error())
				select {
				case <-r.closeCh:
					close(contents)
					return
				case <-cancelCh:
					r.cancelChan.Delete(key)
					close(contents)
					return
				case <-time.After(retryWait):
					continue
				}
			}

			if len(msg.Value) == 0 {
				continue
			}

			select {
			case contents <- msg:
				r.l.Debug(ctxBack, fmt.Sprintf("[consume:%s] received msg, offset: %d", config.Topic, msg.Offset))
			case s := <-sig:
				r.l.Info(ctxBack, fmt.Sprintf("[consume:%s] received signal: %v, exiting...", config.Topic, s))
				close(contents)
				return
			case <-r.closeCh:
				close(contents)
				return
			case <-cancelCh:
				r.cancelChan.Delete(key)
				close(contents)
				return
			}
		}
	}()

	return contents, nil
}

// ConsumeWithHandler 使用回调函数消费消息
func (r *KafkaOperator) ConsumeWithHandler(ctx context.Context, config *ConsumeConfig, handler func(context.Context, kafka.Message) error) error {
	msgCh, err := r.Consume(ctx, config)
	if err != nil {
		return err
	}

	go func() {
		ctxBack := context.Background()
		defer handlePanic(ctxBack, r)

		for msg := range msgCh {
			if handleErr := handler(ctxBack, msg); handleErr != nil {
				r.l.Error(ctxBack, "[consume] handle message failed",
					"err", handleErr.Error(), "topic", msg.Topic, "offset", msg.Offset)
			}
		}
	}()

	return nil
}

// CancelConsume 取消指定 topic+groupID 的消费
func (r *KafkaOperator) CancelConsume(topic, groupID string) error {
	key := topic + ":" + groupID
	val, ok := r.cancelChan.Load(key)
	if !ok {
		return nil
	}

	close(val.(chan bool))

	// 等待消费者退出
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; i < 10; i++ {
		if _, exists := r.cancelChan.Load(key); !exists {
			// 关闭 reader
			if rd, loaded := r.readers.LoadAndDelete(key); loaded {
				_ = rd.(*kafka.Reader).Close()
			}
			return nil
		}
		<-ticker.C
	}

	r.l.Warn(context.Background(), "cancel consumer timeout", "topic", topic, "group_id", groupID)
	return nil
}

// CreateTopic 创建 Topic
func (r *KafkaOperator) CreateTopic(ctx context.Context, topic string, numPartitions, replicationFactor int) error {
	conn, err := kafka.DialContext(ctx, "tcp", r.config.BootstrapServers[0])
	if err != nil {
		return fmt.Errorf("dial kafka failed: %w", err)
	}
	defer conn.Close()

	return conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	})
}

// DeleteTopic 删除 Topic
func (r *KafkaOperator) DeleteTopic(ctx context.Context, topics ...string) error {
	conn, err := kafka.DialContext(ctx, "tcp", r.config.BootstrapServers[0])
	if err != nil {
		return fmt.Errorf("dial kafka failed: %w", err)
	}
	defer conn.Close()

	return conn.DeleteTopics(topics...)
}

// ListTopics 列出所有 Topic
func (r *KafkaOperator) ListTopics(ctx context.Context) ([]string, error) {
	conn, err := kafka.DialContext(ctx, "tcp", r.config.BootstrapServers[0])
	if err != nil {
		return nil, fmt.Errorf("dial kafka failed: %w", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("read partitions failed: %w", err)
	}

	topicSet := make(map[string]struct{})
	for _, p := range partitions {
		topicSet[p.Topic] = struct{}{}
	}

	topics := make([]string, 0, len(topicSet))
	for t := range topicSet {
		topics = append(topics, t)
	}
	return topics, nil
}

// Close 关闭 Kafka 操作器
func (r *KafkaOperator) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if atomic.LoadInt32(&r.closed) == Closed {
		return nil
	}

	atomic.StoreInt32(&r.closed, Closed)

	// 关闭信号
	select {
	case <-r.closeCh:
	default:
		close(r.closeCh)
	}

	// 关闭所有 Writers
	r.writers.Range(func(key, value any) bool {
		if w, ok := value.(*kafka.Writer); ok {
			if err := w.Close(); err != nil {
				r.l.Error(ctx, "close kafka writer failed", "topic", key, "err", err.Error())
			}
		}
		r.writers.Delete(key)
		return true
	})

	// 关闭所有 Readers
	r.readers.Range(func(key, value any) bool {
		if rd, ok := value.(*kafka.Reader); ok {
			if err := rd.Close(); err != nil {
				r.l.Error(ctx, "close kafka reader failed", "key", key, "err", err.Error())
			}
		}
		r.readers.Delete(key)
		return true
	})

	r.l.Info(ctx, "kafka operator closed successfully")
	return nil
}
