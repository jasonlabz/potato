package kafkax

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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

const (
	securityProtocolNone          = "NONE"
	securityProtocolPlaintext     = "PLAINTEXT"
	securityProtocolSSL           = "SSL"
	securityProtocolSASLPlaintext = "SASL_PLAINTEXT"
	securityProtocolSASLSSL       = "SASL_SSL"

	saslMechanismPlain       = "PLAIN"
	saslMechanismScramSHA256 = "SCRAM-SHA-256"
	saslMechanismScramSHA512 = "SCRAM-SHA-512"
	saslMechanismGSSAPI      = "GSSAPI"
	saslMechanismKerberos    = "KERBEROS"
)

// MQConfig Kafka 连接配置
type MQConfig struct {
	BootstrapServers      []string `json:"bootstrap_servers"`
	GroupID               string   `json:"group_id"`
	Topics                []string `json:"topic"`
	SecurityProtocol      string   `json:"security_protocol"`
	SaslMechanism         string   `json:"sasl_mechanism"`
	SaslUsername          string   `json:"sasl_username"`
	SaslPassword          string   `json:"sasl_password"`
	TLSCAFile             string   `json:"tls_ca_file"`
	TLSCertFile           string   `json:"tls_cert_file"`
	TLSKeyFile            string   `json:"tls_key_file"`
	TLSServerName         string   `json:"tls_server_name"`
	TLSInsecureSkipVerify bool     `json:"tls_insecure_skip_verify"`
	MaxAttempts           int      `json:"max_attempts"`
	RetryWaitTime         int64    `json:"retry_wait_time"` // 秒
}

// Validate 验证配置参数
func (c *MQConfig) Validate() error {
	if c == nil {
		return errors.New("kafka config is nil")
	}
	if len(c.BootstrapServers) == 0 {
		return errors.New("bootstrap_servers is empty")
	}
	switch c.securityProtocol() {
	case securityProtocolNone, securityProtocolPlaintext, securityProtocolSSL, securityProtocolSASLPlaintext, securityProtocolSASLSSL:
	default:
		return fmt.Errorf("unsupported security_protocol %q", c.SecurityProtocol)
	}
	if c.MaxAttempts == 0 {
		c.MaxAttempts = DefaultRetryTimes
	}
	if c.RetryWaitTime == 0 {
		c.RetryWaitTime = int64(DefaultRetryWaitTime / time.Second)
	}
	return nil
}

func (c *MQConfig) normalizedSecurityProtocol() string {
	protocol := strings.ToUpper(strings.TrimSpace(c.SecurityProtocol))
	return strings.ReplaceAll(protocol, "-", "_")
}

func (c *MQConfig) securityProtocol() string {
	protocol := c.normalizedSecurityProtocol()
	if protocol == "" {
		return securityProtocolPlaintext
	}
	return protocol
}

func (c *MQConfig) saslMechanismName() string {
	mechanism := strings.ToUpper(strings.TrimSpace(c.SaslMechanism))
	mechanism = strings.ReplaceAll(mechanism, "_", "-")
	return mechanism
}

func (c *MQConfig) usesSASL() bool {
	rawProtocol := c.normalizedSecurityProtocol()
	if rawProtocol == securityProtocolNone || rawProtocol == securityProtocolPlaintext {
		return false
	}
	protocol := c.securityProtocol()
	return protocol == securityProtocolSASLPlaintext || protocol == securityProtocolSASLSSL || c.saslMechanismName() != ""
}

func (c *MQConfig) usesTLS() bool {
	rawProtocol := c.normalizedSecurityProtocol()
	if rawProtocol == securityProtocolNone || rawProtocol == securityProtocolPlaintext {
		return false
	}
	protocol := c.securityProtocol()
	return protocol == securityProtocolSSL ||
		protocol == securityProtocolSASLSSL ||
		c.TLSCAFile != "" ||
		c.TLSCertFile != "" ||
		c.TLSKeyFile != "" ||
		c.TLSServerName != ""
}

// getSASLMechanism 获取 SASL 认证机制
func (c *MQConfig) getSASLMechanism() (sasl.Mechanism, error) {
	if !c.usesSASL() {
		return nil, nil
	}
	if c.saslMechanismName() == "" {
		return nil, errors.New("sasl_mechanism is required when SASL is enabled")
	}

	switch c.saslMechanismName() {
	case saslMechanismPlain:
		return &plain.Mechanism{
			Username: c.SaslUsername,
			Password: c.SaslPassword,
		}, nil
	case saslMechanismScramSHA256:
		return scram.Mechanism(scram.SHA256, c.SaslUsername, c.SaslPassword)
	case saslMechanismScramSHA512:
		return scram.Mechanism(scram.SHA512, c.SaslUsername, c.SaslPassword)
	case saslMechanismGSSAPI, saslMechanismKerberos:
		return nil, errors.New("SASL GSSAPI/Kerberos is not implemented by kafkax config; pass a kafka-go sasl.Mechanism with WithSASLMechanism")
	default:
		return nil, fmt.Errorf("unsupported sasl_mechanism %q", c.SaslMechanism)
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

	transport kafka.RoundTripper
	closeCh   chan bool
	closed    int32
	mu        sync.Mutex
	l         log.Logger
}

// NewKafkaOperator 创建 Kafka 操作器实例
func NewKafkaOperator(config *MQConfig, opts ...ConnOption) (op *KafkaOperator, err error) {
	if config == nil {
		return nil, errors.New("kafka config is nil")
	}
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

	mechanism, err := r.saslMechanism()
	if err != nil {
		return nil, fmt.Errorf("create SASL mechanism failed: %w", err)
	}
	if mechanism != nil {
		t.SASL = mechanism
	}

	tlsConfig, err := r.tlsConfig()
	if err != nil {
		return nil, fmt.Errorf("create TLS config failed: %w", err)
	}
	if tlsConfig != nil {
		t.TLS = tlsConfig
	}

	return t, nil
}

func (r *KafkaOperator) saslMechanism() (sasl.Mechanism, error) {
	if r.connCfg != nil && r.connCfg.customSASLMechanism != nil {
		return r.connCfg.customSASLMechanism, nil
	}
	return r.config.getSASLMechanism()
}

func (r *KafkaOperator) tlsConfig() (*tls.Config, error) {
	if r.connCfg != nil && r.connCfg.customTLSConfig != nil {
		return r.connCfg.customTLSConfig, nil
	}
	if !r.config.usesTLS() {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         r.config.TLSServerName,
		InsecureSkipVerify: r.config.TLSInsecureSkipVerify,
	}

	if r.config.TLSCAFile != "" {
		caPEM, err := os.ReadFile(r.config.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read tls_ca_file failed: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("tls_ca_file does not contain any PEM certificates")
		}
		tlsConfig.RootCAs = pool
	}

	if r.config.TLSCertFile != "" || r.config.TLSKeyFile != "" {
		if r.config.TLSCertFile == "" || r.config.TLSKeyFile == "" {
			return nil, errors.New("tls_cert_file and tls_key_file must be configured together")
		}
		cert, err := tls.LoadX509KeyPair(r.config.TLSCertFile, r.config.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load tls client certificate failed: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func (r *KafkaOperator) buildDialer() (*kafka.Dialer, error) {
	mechanism, err := r.saslMechanism()
	if err != nil {
		return nil, fmt.Errorf("create SASL mechanism failed: %w", err)
	}
	tlsConfig, err := r.tlsConfig()
	if err != nil {
		return nil, fmt.Errorf("create TLS config failed: %w", err)
	}
	return &kafka.Dialer{
		Timeout:       DefaultTimeOut,
		SASLMechanism: mechanism,
		TLS:           tlsConfig,
	}, nil
}

func (r *KafkaOperator) dial(ctx context.Context) (*kafka.Conn, error) {
	dialer, err := r.buildDialer()
	if err != nil {
		return nil, err
	}
	return dialer.DialContext(ctx, "tcp", r.config.BootstrapServers[0])
}

// ping 验证 Kafka 连接
func (r *KafkaOperator) ping(ctx context.Context) error {
	conn, err := r.dial(ctx)
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
		Addr:     kafka.TCP(r.config.BootstrapServers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
		// 等待所有 ISR 确认，避免 leader 宕机时已返回成功的消息丢失。
		RequiredAcks: kafka.RequireAll,
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
func (r *KafkaOperator) getReader(config *ConsumeConfig) (*kafka.Reader, error) {
	key := config.Topic + ":" + config.GroupID
	if rd, ok := r.readers.Load(key); ok {
		return rd.(*kafka.Reader), nil
	}

	r.readersMu.Lock()
	defer r.readersMu.Unlock()

	if rd, ok := r.readers.Load(key); ok {
		return rd.(*kafka.Reader), nil
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

	dialer, err := r.buildDialer()
	if err != nil {
		return nil, err
	}
	readerCfg.Dialer = dialer

	rd := kafka.NewReader(readerCfg)
	r.readers.Store(key, rd)
	return rd, nil
}

// Push 推送单条消息
func (r *KafkaOperator) Push(ctx context.Context, msg *ProduceMessage) error {
	if msg == nil {
		return errors.New("message is nil")
	}
	return r.PushBatch(ctx, msg.Topic, []*ProduceMessage{msg})
}

// PushBatch 批量推送消息
func (r *KafkaOperator) PushBatch(ctx context.Context, topic string, messages []*ProduceMessage) (err error) {
	defer handlePanic(ctx, r, &err)

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
		// Writer 按 topic 创建，消息自带的 Topic 不一致时报错，避免静默写到别的 topic。
		if m.Topic != "" && m.Topic != topic {
			return fmt.Errorf("message topic %q does not match batch topic %q", m.Topic, topic)
		}
		kafkaMessages = append(kafkaMessages, m.toKafkaMessage())
	}

	// 带重试的推送
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

// Consume 消费消息，返回消息通道。有 GroupID 时消息写入通道后才提交 offset。
func (r *KafkaOperator) Consume(ctx context.Context, config *ConsumeConfig) (<-chan kafka.Message, error) {
	msgCh, _, err := r.consume(ctx, config, true)
	return msgCh, err
}

// consume 启动消费协程，返回的 commit 绑定本次消费的 reader；autoCommit 为 false 时
// 由调用方在处理成功后调用 commit。
func (r *KafkaOperator) consume(_ context.Context, config *ConsumeConfig, autoCommit bool) (<-chan kafka.Message, func(kafka.Message), error) {
	if atomic.LoadInt32(&r.closed) == Closed {
		return nil, nil, errors.New("kafka operator is closed")
	}

	if config == nil {
		return nil, nil, errors.New("consume config is nil")
	}
	if config.Topic == "" {
		return nil, nil, errors.New("topic is required")
	}

	key := config.Topic + ":" + config.GroupID
	// 创建取消通道；同一 topic+groupID 已在消费时报错，避免覆盖后旧消费者无法取消。
	cancelCh := make(chan bool)
	if _, loaded := r.cancelChan.LoadOrStore(key, cancelCh); loaded {
		return nil, nil, fmt.Errorf("topic %s group %s is already being consumed", config.Topic, config.GroupID)
	}
	reader, err := r.getReader(config)
	if err != nil {
		r.cancelChan.CompareAndDelete(key, cancelCh)
		return nil, nil, err
	}
	commit := r.committer(reader, cancelCh)
	contents := make(chan kafka.Message, 10)

	go func() {
		ctxBack := context.Background()
		defer handlePanic(ctxBack, r, nil)
		defer close(contents)
		defer r.cancelChan.CompareAndDelete(key, cancelCh)

		retryWait := time.Duration(r.config.RetryWaitTime) * time.Second

		for {
			select {
			case <-r.closeCh:
				r.l.Info(ctxBack, fmt.Sprintf("[consume:%s] kafka is closed, exiting...", config.Topic))
				return
			case <-cancelCh:
				r.l.Info(ctxBack, fmt.Sprintf("[consume:%s] consume canceled, exiting...", config.Topic))
				return
			default:
			}

			// FetchMessage 不自动提交；CancelConsume/Close 关闭 reader 时会立即返回错误。
			msg, err := reader.FetchMessage(ctxBack)
			if err != nil {
				if atomic.LoadInt32(&r.closed) == Closed {
					return
				}
				r.l.Error(ctxBack, fmt.Sprintf("[consume:%s] read message error, retrying...", config.Topic),
					"err", err.Error())
				select {
				case <-r.closeCh:
					return
				case <-cancelCh:
					return
				case <-time.After(retryWait):
					continue
				}
			}

			if len(msg.Value) == 0 {
				if autoCommit {
					commit(msg)
				}
				continue
			}

			select {
			case contents <- msg:
				r.l.Debug(ctxBack, fmt.Sprintf("[consume:%s] received msg, offset: %d", config.Topic, msg.Offset))
				if autoCommit {
					commit(msg)
				}
			case <-r.closeCh:
				return
			case <-cancelCh:
				return
			}
		}
	}()

	return contents, commit, nil
}

// committer 返回绑定 reader 的同步提交函数，没有 GroupID 时为空操作。提交用的 ctx 在
// Close 或 CancelConsume 时取消，避免 reader 已关闭时 CommitMessages 一直阻塞。
// 提交失败只记日志（消息会在重平衡后重新投递）。
func (r *KafkaOperator) committer(reader *kafka.Reader, cancelCh <-chan bool) func(kafka.Message) {
	if reader.Config().GroupID == "" {
		return func(kafka.Message) {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		select {
		case <-r.closeCh:
		case <-cancelCh:
		}
	}()
	return func(msg kafka.Message) {
		if err := reader.CommitMessages(ctx, msg); err != nil {
			r.l.Error(ctx, "[consume] commit offset failed", "err", err.Error(), "topic", msg.Topic, "offset", msg.Offset)
		}
	}
}

// ConsumeWithHandler 使用回调函数消费消息；有 GroupID 时 handler 成功后才提交 offset。
func (r *KafkaOperator) ConsumeWithHandler(ctx context.Context, config *ConsumeConfig, handler func(context.Context, kafka.Message) error) error {
	msgCh, commit, err := r.consume(ctx, config, false)
	if err != nil {
		return err
	}

	go func() {
		ctxBack := context.Background()
		defer handlePanic(ctxBack, r, nil)

		for msg := range msgCh {
			if handleErr := handler(ctxBack, msg); handleErr != nil {
				r.l.Error(ctxBack, "[consume] handle message failed",
					"err", handleErr.Error(), "topic", msg.Topic, "offset", msg.Offset)
				continue
			}
			commit(msg)
		}
	}()

	return nil
}

// CancelConsume 取消指定 topic+groupID 的消费，并关闭对应 reader 以打断阻塞中的读取。
// 先移除 reader 再移除取消通道：取消通道仍在时并发的 Consume 会报重复消费，
// 不会拿到即将被关闭的旧 reader。
func (r *KafkaOperator) CancelConsume(topic, groupID string) error {
	key := topic + ":" + groupID
	if _, ok := r.cancelChan.Load(key); !ok {
		return nil
	}

	var err error
	if rd, loaded := r.readers.LoadAndDelete(key); loaded {
		err = rd.(*kafka.Reader).Close()
	}
	if val, ok := r.cancelChan.LoadAndDelete(key); ok {
		close(val.(chan bool))
	}
	return err
}

// CreateTopic 创建 Topic
func (r *KafkaOperator) CreateTopic(ctx context.Context, topic string, numPartitions, replicationFactor int) error {
	conn, err := r.dial(ctx)
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
	conn, err := r.dial(ctx)
	if err != nil {
		return fmt.Errorf("dial kafka failed: %w", err)
	}
	defer conn.Close()

	return conn.DeleteTopics(topics...)
}

// ListTopics 列出所有 Topic
func (r *KafkaOperator) ListTopics(ctx context.Context) ([]string, error) {
	client := r.adminClient()
	metadata, err := client.Metadata(ctx, &kafka.MetadataRequest{})
	if err != nil {
		return nil, fmt.Errorf("read metadata failed: %w", err)
	}

	topicSet := make(map[string]struct{}, len(metadata.Topics))
	for _, topic := range metadata.Topics {
		if topic.Error != nil {
			return nil, fmt.Errorf("read topic metadata %q failed: %w", topic.Name, topic.Error)
		}
		topicSet[topic.Name] = struct{}{}
	}

	topics := make([]string, 0, len(topicSet))
	for topic := range topicSet {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	return topics, nil
}

func (r *KafkaOperator) PreviewMessages(ctx context.Context, topic string, limit int) ([]kafka.Message, error) {
	if limit <= 0 {
		limit = 10
	}
	client := r.adminClient()
	metadata, err := client.Metadata(ctx, &kafka.MetadataRequest{Topics: []string{topic}})
	if err != nil {
		return nil, fmt.Errorf("read topic metadata %q failed: %w", topic, err)
	}
	if len(metadata.Topics) == 0 {
		return nil, nil
	}
	partitions := metadata.Topics[0].Partitions
	if len(partitions) == 0 {
		return nil, nil
	}

	offsetReqs := make([]kafka.OffsetRequest, 0, len(partitions)*2)
	for _, partition := range partitions {
		offsetReqs = append(offsetReqs, kafka.FirstOffsetOf(partition.ID), kafka.LastOffsetOf(partition.ID))
	}
	offsets, err := client.ListOffsets(ctx, &kafka.ListOffsetsRequest{Topics: map[string][]kafka.OffsetRequest{topic: offsetReqs}})
	if err != nil {
		return nil, fmt.Errorf("list topic offsets %q failed: %w", topic, err)
	}

	boundsByPartition := map[int]struct{ first, last int64 }{}
	for _, partition := range offsets.Topics[topic] {
		if partition.Error != nil {
			return nil, fmt.Errorf("read partition %d offsets failed: %w", partition.Partition, partition.Error)
		}
		boundsByPartition[partition.Partition] = struct{ first, last int64 }{first: partition.FirstOffset, last: partition.LastOffset}
	}

	messages := make([]kafka.Message, 0, limit)
	for _, partition := range partitions {
		bounds := boundsByPartition[partition.ID]
		if bounds.last <= bounds.first {
			continue
		}
		start := bounds.first
		if count := bounds.last - bounds.first; count > int64(limit-len(messages)) {
			start = bounds.last - int64(limit-len(messages))
		}
		fetched, err := client.Fetch(ctx, &kafka.FetchRequest{Topic: topic, Partition: partition.ID, Offset: start, MinBytes: 1, MaxBytes: 1024 * 1024, MaxWait: 100 * time.Millisecond})
		if err != nil {
			return nil, fmt.Errorf("fetch preview messages %q partition %d failed: %w", topic, partition.ID, err)
		}
		if fetched.Error != nil {
			return nil, fmt.Errorf("fetch preview messages %q partition %d failed: %w", topic, partition.ID, fetched.Error)
		}
		for len(messages) < limit {
			record, err := fetched.Records.ReadRecord()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("read preview record %q partition %d failed: %w", topic, partition.ID, err)
			}
			if record.Offset < start {
				continue
			}
			key, err := kafka.ReadAll(record.Key)
			if err != nil {
				return nil, fmt.Errorf("read preview key %q partition %d offset %d failed: %w", topic, partition.ID, record.Offset, err)
			}
			value, err := kafka.ReadAll(record.Value)
			if err != nil {
				return nil, fmt.Errorf("read preview value %q partition %d offset %d failed: %w", topic, partition.ID, record.Offset, err)
			}
			messages = append(messages, kafka.Message{Topic: topic, Partition: partition.ID, Offset: record.Offset, Key: key, Value: value, Headers: record.Headers})
		}
		if len(messages) >= limit {
			break
		}
	}
	return messages, nil
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
