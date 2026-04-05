package rocketmqx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/google/uuid"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/utils"
)

var operator *RocketMQOperator

// GetRocketMQOperator 获取全局 RocketMQ 操作器实例
func GetRocketMQOperator() *RocketMQOperator {
	return operator
}

func init() {
	appConf := configx.GetConfig()
	if appConf.RocketMQ.Enable {
		mqConf := &MQConfig{}
		err := utils.CopyStruct(appConf.RocketMQ, mqConf)
		if err != nil {
			zapx.GetLogger().WithError(err).Error(context.Background(), "copy rocketmq config error, skipping ...")
			return
		}
		operator, err = NewRocketMQOperator(mqConf)
		if err == nil {
			return
		}
		zapx.GetLogger().WithError(err).Error(context.Background(), "init rocketmq Client error")
		if appConf.RocketMQ.Strict {
			panic(fmt.Errorf("init rocketmq Client error: %v", err))
		}
	}
}

const (
	DefaultRetryTimes          = 3
	DefaultRetryWaitTime       = 2 * time.Second
	Closed               int32 = 1
)

// MQConfig RocketMQ 连接配置
type MQConfig struct {
	NameServers   []string `json:"name_servers"`
	GroupName     string   `json:"group_name"`
	AccessKey     string   `json:"access_key"`
	SecretKey     string   `json:"secret_key"`
	Namespace     string   `json:"namespace"`
	RetryTimes    int      `json:"retry_times"`
	RetryWaitTime int64    `json:"retry_wait_time"` // 秒
}

// Validate 验证配置参数
func (c *MQConfig) Validate() error {
	if len(c.NameServers) == 0 {
		return errors.New("name_servers is empty")
	}
	if c.GroupName == "" {
		c.GroupName = "DEFAULT_GROUP"
	}
	if c.RetryTimes == 0 {
		c.RetryTimes = DefaultRetryTimes
	}
	if c.RetryWaitTime == 0 {
		c.RetryWaitTime = int64(DefaultRetryWaitTime / time.Second)
	}
	return nil
}

// getCredentials 获取认证凭证
func (c *MQConfig) getCredentials() *primitive.Credentials {
	if c.AccessKey == "" && c.SecretKey == "" {
		return nil
	}
	return &primitive.Credentials{
		AccessKey: c.AccessKey,
		SecretKey: c.SecretKey,
	}
}

// RocketMQOperator RocketMQ 操作器
type RocketMQOperator struct {
	name    string
	config  *MQConfig
	connCfg *ConnConfig

	producer  rocketmq.Producer
	consumers sync.Map // topic+groupName -> rocketmq.PushConsumer
	cancelMu  sync.Mutex

	closeCh chan bool
	closed  int32
	mu      sync.Mutex
	l       log.Logger
}

// NewRocketMQOperator 创建 RocketMQ 操作器实例
func NewRocketMQOperator(config *MQConfig, opts ...ConnOption) (op *RocketMQOperator, err error) {
	if err = config.Validate(); err != nil {
		return
	}

	connCfg := DefaultConfig()
	for _, opt := range opts {
		opt(connCfg)
	}

	ctx := context.Background()
	op = &RocketMQOperator{
		name:    uuid.NewString(),
		config:  config,
		connCfg: connCfg,
		closeCh: make(chan bool),
		l:       connCfg.l,
	}

	// 创建 Producer
	if err = op.initProducer(); err != nil {
		return nil, fmt.Errorf("init rocketmq producer failed: %w", err)
	}

	op.l.Info(ctx, "rocketmq operator created successfully", "name_servers", config.NameServers)
	return
}

// initProducer 初始化 Producer
func (r *RocketMQOperator) initProducer() error {
	opts := []producer.Option{
		producer.WithNameServer(r.config.NameServers),
		producer.WithGroupName(r.config.GroupName),
		producer.WithRetry(r.connCfg.RetryTimes),
		producer.WithSendMsgTimeout(r.connCfg.SendTimeout),
	}

	if r.config.Namespace != "" {
		opts = append(opts, producer.WithNamespace(r.config.Namespace))
	}

	creds := r.config.getCredentials()
	if creds != nil {
		opts = append(opts, producer.WithCredentials(*creds))
	}

	p, err := rocketmq.NewProducer(opts...)
	if err != nil {
		return fmt.Errorf("create producer failed: %w", err)
	}

	if err = p.Start(); err != nil {
		return fmt.Errorf("start producer failed: %w", err)
	}

	r.producer = p
	return nil
}

// createPushConsumer 创建 PushConsumer
func (r *RocketMQOperator) createPushConsumer(config *ConsumeConfig) (rocketmq.PushConsumer, error) {
	groupName := config.GroupName
	if groupName == "" {
		groupName = r.config.GroupName
	}

	opts := []consumer.Option{
		consumer.WithNameServer(r.config.NameServers),
		consumer.WithGroupName(groupName),
		consumer.WithConsumeGoroutineNums(r.connCfg.ConsumeGoroutineNums),
		consumer.WithMaxReconsumeTimes(int32(r.connCfg.MaxReconsumeTimes)),
		consumer.WithPullBatchSize(int32(r.connCfg.PullBatchSize)),
		consumer.WithRetry(r.connCfg.RetryTimes),
	}

	if r.config.Namespace != "" {
		opts = append(opts, consumer.WithNamespace(r.config.Namespace))
	}

	creds := r.config.getCredentials()
	if creds != nil {
		opts = append(opts, consumer.WithCredentials(*creds))
	}

	// 消费模式
	switch config.ConsumerModel {
	case BroadCasting:
		opts = append(opts, consumer.WithConsumerModel(consumer.BroadCasting))
	default:
		opts = append(opts, consumer.WithConsumerModel(consumer.Clustering))
	}

	// 消费起始位置
	switch config.ConsumeFrom {
	case ConsumeFromFirstOffset:
		opts = append(opts, consumer.WithConsumeFromWhere(consumer.ConsumeFromFirstOffset))
	case ConsumeFromTimestamp:
		opts = append(opts, consumer.WithConsumeFromWhere(consumer.ConsumeFromTimestamp))
	default:
		opts = append(opts, consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset))
	}

	// 顺序消费
	if config.OrderConsume {
		opts = append(opts, consumer.WithConsumerOrder(true))
	}

	c, err := rocketmq.NewPushConsumer(opts...)
	if err != nil {
		return nil, fmt.Errorf("create push consumer failed: %w", err)
	}

	return c, nil
}

// Push 推送单条消息（同步）
func (r *RocketMQOperator) Push(ctx context.Context, msg *ProduceMessage) (*primitive.SendResult, error) {
	return r.PushBatch(ctx, []*ProduceMessage{msg})
}

// PushBatch 批量推送消息（同步）
func (r *RocketMQOperator) PushBatch(ctx context.Context, messages []*ProduceMessage) (*primitive.SendResult, error) {
	defer handlePanic(ctx, r)

	if atomic.LoadInt32(&r.closed) == Closed {
		return nil, errors.New("rocketmq operator is closed")
	}

	if len(messages) == 0 {
		return nil, errors.New("messages is empty")
	}

	msgs := make([]*primitive.Message, 0, len(messages))
	for _, m := range messages {
		if m.Topic == "" {
			return nil, errors.New("topic is required")
		}
		msgs = append(msgs, m.toPrimitiveMessage())
	}

	// 带重试的推送
	var result *primitive.SendResult
	var err error
	retryWait := time.Duration(r.config.RetryWaitTime) * time.Second

	for i := 0; i < r.config.RetryTimes; i++ {
		if atomic.LoadInt32(&r.closed) == Closed {
			return nil, errors.New("rocketmq operator is closed")
		}

		result, err = r.producer.SendSync(ctx, msgs...)
		if err == nil {
			r.l.Info(ctx, "[push] rocketmq push success",
				"topic", messages[0].Topic, "msg_id", result.MsgID, "count", len(messages))
			return result, nil
		}

		r.l.Warn(ctx, "[push] rocketmq push failed, retrying...",
			"err", err.Error(), "topic", messages[0].Topic, "retry_count", i+1)

		select {
		case <-r.closeCh:
			return nil, errors.New("rocketmq operator closed during push")
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryWait):
		}
	}

	r.l.Error(ctx, "[push] rocketmq push failed after retries",
		"err", err.Error(), "topic", messages[0].Topic, "attempts", r.config.RetryTimes)
	return nil, fmt.Errorf("push failed after %d retries: %w", r.config.RetryTimes, err)
}

// PushAsync 异步推送消息
func (r *RocketMQOperator) PushAsync(ctx context.Context, msg *ProduceMessage,
	callback func(ctx context.Context, result *primitive.SendResult, err error)) error {
	defer handlePanic(ctx, r)

	if atomic.LoadInt32(&r.closed) == Closed {
		return errors.New("rocketmq operator is closed")
	}

	if msg.Topic == "" {
		return errors.New("topic is required")
	}

	return r.producer.SendAsync(ctx, callback, msg.toPrimitiveMessage())
}

// PushOneWay 单向推送（不等待响应）
func (r *RocketMQOperator) PushOneWay(ctx context.Context, msg *ProduceMessage) error {
	defer handlePanic(ctx, r)

	if atomic.LoadInt32(&r.closed) == Closed {
		return errors.New("rocketmq operator is closed")
	}

	if msg.Topic == "" {
		return errors.New("topic is required")
	}

	return r.producer.SendOneWay(ctx, msg.toPrimitiveMessage())
}

// PushDelay 推送延迟消息
func (r *RocketMQOperator) PushDelay(ctx context.Context, msg *ProduceMessage, delayLevel int) (*primitive.SendResult, error) {
	msg.SetDelayLevel(delayLevel)
	return r.Push(ctx, msg)
}

// Consume 消费消息（PushConsumer 模式）
func (r *RocketMQOperator) Consume(ctx context.Context, config *ConsumeConfig,
	handler func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
	if atomic.LoadInt32(&r.closed) == Closed {
		return errors.New("rocketmq operator is closed")
	}

	if config.Topic == "" {
		return errors.New("topic is required")
	}

	groupName := config.GroupName
	if groupName == "" {
		groupName = r.config.GroupName
	}
	key := config.Topic + ":" + groupName

	c, err := r.createPushConsumer(config)
	if err != nil {
		return fmt.Errorf("create consumer failed: %w", err)
	}

	// 构建 tag 选择器
	selector := consumer.MessageSelector{}
	if config.Tags != "" {
		selector.Type = consumer.TAG
		selector.Expression = config.Tags
	}

	// 订阅
	if err = c.Subscribe(config.Topic, selector, handler); err != nil {
		return fmt.Errorf("subscribe failed: %w", err)
	}

	// 启动消费者
	if err = c.Start(); err != nil {
		return fmt.Errorf("start consumer failed: %w", err)
	}

	r.consumers.Store(key, c)
	r.l.Info(ctx, "[consume] rocketmq consumer started",
		"topic", config.Topic, "group", groupName)

	return nil
}

// ConsumeWithChannel 消费消息，返回消息通道（包装 PushConsumer）
func (r *RocketMQOperator) ConsumeWithChannel(ctx context.Context, config *ConsumeConfig) (<-chan []*primitive.MessageExt, error) {
	msgCh := make(chan []*primitive.MessageExt, 10)

	err := r.Consume(ctx, config, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		select {
		case msgCh <- msgs:
			return consumer.ConsumeSuccess, nil
		case <-r.closeCh:
			close(msgCh)
			return consumer.ConsumeRetryLater, nil
		}
	})
	if err != nil {
		close(msgCh)
		return nil, err
	}

	return msgCh, nil
}

// CancelConsume 取消消费
func (r *RocketMQOperator) CancelConsume(topic, groupName string) error {
	if groupName == "" {
		groupName = r.config.GroupName
	}
	key := topic + ":" + groupName

	val, ok := r.consumers.LoadAndDelete(key)
	if !ok {
		return nil
	}

	c := val.(rocketmq.PushConsumer)
	if err := c.Shutdown(); err != nil {
		return fmt.Errorf("shutdown consumer failed: %w", err)
	}

	r.l.Info(context.Background(), "[consume] consumer stopped",
		"topic", topic, "group", groupName)
	return nil
}

// Close 关闭 RocketMQ 操作器
func (r *RocketMQOperator) Close(ctx context.Context) error {
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

	// 关闭 Producer
	if r.producer != nil {
		if err := r.producer.Shutdown(); err != nil {
			r.l.Error(ctx, "shutdown rocketmq producer failed", "err", err.Error())
		}
	}

	// 关闭所有 Consumers
	r.consumers.Range(func(key, value any) bool {
		if c, ok := value.(rocketmq.PushConsumer); ok {
			if err := c.Shutdown(); err != nil {
				r.l.Error(ctx, "shutdown rocketmq consumer failed", "key", key, "err", err.Error())
			}
		}
		r.consumers.Delete(key)
		return true
	})

	r.l.Info(ctx, "rocketmq operator closed successfully")
	return nil
}
