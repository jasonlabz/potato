package rabbitmqx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jolestar/go-commons-pool/v2"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/times"
	"github.com/jasonlabz/potato/utils"
)

var operator *RabbitMQOperator

// GetRabbitMQOperator 获取全局 RabbitMQ 操作器实例
func GetRabbitMQOperator() *RabbitMQOperator {
	if operator == nil {
		panic("rabbitmq has not init")
	}
	return operator
}

func init() {
	appConf := configx.GetConfig()
	if appConf.Rabbitmq.Enable {
		mqConf := &MQConfig{}
		err := utils.CopyStruct(appConf.Rabbitmq, mqConf)
		if err != nil {
			zapx.GetLogger().WithError(err).Error(context.Background(), "copy rmq config error, skipping ...")
			return
		}
		err = InitRabbitMQOperator(mqConf)
		if err != nil {
			zapx.GetLogger().WithError(err).Error(context.Background(), "init rmq Client error, skipping ...")
		}
	}
}

func genUUID() string {
	return uuid.NewString()
}

// InitRabbitMQOperator 负责初始化全局变量operator
func InitRabbitMQOperator(config *MQConfig) (err error) {
	operator, err = NewRabbitMQOperator(config)
	return
}

const (
	DefaultTimeOut           = 2 * time.Second
	DefaultRetryWaitTimes    = 2 * time.Second
	DefaultConsumeRetryTimes = 3 * time.Second
	Closed                   = 1
	RetryTimes               = 3
)

type ExchangeType string

const (
	Direct ExchangeType = "direct" // 直连
	Fanout ExchangeType = "fanout" // 广播
	Topic  ExchangeType = "topic"  // 通配符
)

// MQConfig 定义队列连接信息
type MQConfig struct {
	Username    string    `json:"username"` // 用户
	Password    string    `json:"password"` // 密码
	Host        string    `json:"host"`     // 服务地址
	Port        int       `json:"port"`     // 端口
	LimitSwitch bool      `json:"limit_switch"`
	LimitConf   LimitConf `json:"limit_conf"`
}

type LimitConf struct {
	AttemptTimes    int `json:"attempt_times"`
	RetryTimeSecond int `json:"retry_time_second"`
	PrefetchCount   int `json:"prefetch_count"`
	Timeout         int `json:"timeout"`
	QueueLimit      int `json:"queue_limit"`
}

// QueueLockManager 队列锁管理器，确保同一队列的并发操作安全
type QueueLockManager struct {
	locks sync.Map // 存储队列名称对应的互斥锁
	mu    sync.Mutex
}

// GetLock 获取指定队列的锁，如果不存在则创建
func (q *QueueLockManager) GetLock(queueName string) *sync.Mutex {
	q.mu.Lock()
	defer q.mu.Unlock()

	if lock, ok := q.locks.Load(queueName); ok {
		return lock.(*sync.Mutex)
	}

	lock := &sync.Mutex{}
	q.locks.Store(queueName, lock)
	return lock
}

// RemoveLock 移除指定队列的锁
func (q *QueueLockManager) RemoveLock(queueName string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.locks.Delete(queueName)
}

// ChannelWrap 通道包装器，封装 AMQP 通道和相关状态
type ChannelWrap struct {
	channel     *amqp.Channel
	openConfirm bool                   // 当isConsumer为false，即推送channel是否打开确认模式
	isConsumer  bool                   // 是否为消费队列使用
	confirmChan chan amqp.Confirmation // 用于推送channel的消息推送确认
	closeChan   chan *amqp.Error       // 用于消费channel的监听
}

// Close 安全关闭通道
func (c *ChannelWrap) Close() error {
	if c.channel == nil || c.channel.IsClosed() {
		return nil
	}
	return c.channel.Close()
}

// Validate 验证配置参数
func (c *MQConfig) Validate() error {
	if c.Host == "" {
		return errors.New("host is empty")
	}
	if c.Port == 0 {
		return errors.New("port is empty")
	}
	return nil
}

// addr 构建连接地址
func (c *MQConfig) addr() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", c.Username, c.Password, c.Host, c.Port)
}

// RabbitMQOperator RabbitMQ 操作器，提供消息推送和消费功能
type RabbitMQOperator struct {
	name       string
	config     *MQConfig
	client     *Client
	opCache    sync.Map          // 操作缓存，避免重复声明
	closeCh    chan bool         // 关闭通道
	isReady    bool              // 连接就绪状态
	closed     int32             // 关闭状态原子标记
	mu         sync.Mutex        // 操作器级别互斥锁
	declareMu  sync.Mutex        // 声明操作互斥锁
	l          log.Logger        // 日志记录器
	queueLocks *QueueLockManager // 队列级别锁管理器
}

// Client RabbitMQ 客户端，管理连接和通道池
type Client struct {
	conn            *amqp.Connection // AMQP 连接
	chanPool        *pool.ObjectPool // 普通通道池
	confirmChanPool *pool.ObjectPool // 确认模式通道池
	cancelChan      sync.Map         // 取消通道映射
	cacheMu         sync.Mutex       // 缓存互斥锁
	connMu          sync.Mutex       // 连接互斥锁
	closeConnNotify chan *amqp.Error // 连接关闭通知
}

// NewRabbitMQOperator 创建 RabbitMQ 操作器实例
func NewRabbitMQOperator(config *MQConfig, opts ...ConnOption) (op *RabbitMQOperator, err error) {
	// 验证配置参数
	if err = config.Validate(); err != nil {
		return
	}

	ctx := context.Background()
	defaultConfig := DefaultConfig()
	for _, opt := range opts {
		opt(defaultConfig)
	}

	// 初始化操作器
	op = &RabbitMQOperator{
		config:     config,
		client:     &Client{closeConnNotify: make(chan *amqp.Error, 1)},
		closeCh:    make(chan bool),
		name:       genUUID(),
		l:          defaultConfig.l,
		queueLocks: &QueueLockManager{},
	}

	// 初始化通道池
	op.init(ctx, &defaultConfig.ObjectPoolConfig)

	// 建立连接
	if err = op.connect(ctx); err != nil {
		return
	}

	// 启动重连守护协程 - 修复空指针问题
	go func() {
		defer func() {
			if r := recover(); r != nil {
				op.l.Error(ctx, "daemon reconnection goroutine panic", "recover", r)
			}
		}()
		_, connectErr := op.tryReConnect(true)
		if connectErr != nil {
			op.l.Warn(ctx, "daemon progress of reconnection has exited", "error", connectErr.Error())
		}
	}()

	return
}

// init 初始化通道池
func (r *RabbitMQOperator) init(ctx context.Context, config *pool.ObjectPoolConfig) {
	// 普通通道工厂
	factory := pool.NewPooledObjectFactory(
		func(ctx context.Context) (any, error) {
			return r.createChannel(false)
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			return r.closeChannel(object)
		},
		func(ctx context.Context, object *pool.PooledObject) bool {
			return r.validateChannel(object)
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			return r.activateChannel(object)
		},
		nil,
	)

	// 确认模式通道工厂
	confirmFactory := pool.NewPooledObjectFactory(
		func(ctx context.Context) (any, error) {
			return r.createChannel(true)
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			return r.closeChannel(object)
		},
		func(ctx context.Context, object *pool.PooledObject) bool {
			return r.validateChannel(object)
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			return r.activateChannel(object)
		},
		nil,
	)

	// 创建通道池
	r.client.confirmChanPool = pool.NewObjectPool(ctx, confirmFactory, config)
	r.client.chanPool = pool.NewObjectPool(ctx, factory, config)
}

// createChannel 创建通道的通用方法
func (r *RabbitMQOperator) createChannel(openConfirm bool) (*ChannelWrap, error) {
	// 检查连接状态，必要时重连
	if !r.isReady && atomic.LoadInt32(&r.closed) != Closed {
		if connected, connectErr := r.tryReConnect(false); !connected {
			return nil, connectErr
		}
	}

	// 检查连接是否有效
	if r.client.conn == nil || r.client.conn.IsClosed() {
		return nil, errors.New("rabbitmq connection is not available")
	}

	// 创建通道
	channel, err := r.client.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("create channel failed: %w", err)
	}

	channelWrap := &ChannelWrap{
		channel:    channel,
		closeChan:  make(chan *amqp.Error),
		isConsumer: false,
	}

	// 如果开启确认模式，进行相应配置
	if openConfirm {
		if err := channel.Confirm(false); err != nil {
			channel.Close()
			return nil, fmt.Errorf("enable confirm mode failed: %w", err)
		}
		channelWrap.openConfirm = true
		channelWrap.confirmChan = make(chan amqp.Confirmation, 20) // 增加缓冲区大小避免阻塞
		channel.NotifyPublish(channelWrap.confirmChan)
	}

	// 设置通道关闭通知
	channel.NotifyClose(channelWrap.closeChan)

	return channelWrap, nil
}

// closeChannel 关闭通道的通用方法
func (r *RabbitMQOperator) closeChannel(object *pool.PooledObject) error {
	channelWrap, ok := object.Object.(*ChannelWrap)
	if !ok || channelWrap == nil {
		return nil
	}
	return channelWrap.Close()
}

// validateChannel 验证通道状态的通用方法
func (r *RabbitMQOperator) validateChannel(object *pool.PooledObject) bool {
	channelWrap, ok := object.Object.(*ChannelWrap)
	if !ok || channelWrap == nil {
		return false
	}
	return channelWrap.channel != nil && !channelWrap.channel.IsClosed()
}

// activateChannel 激活通道的通用方法
func (r *RabbitMQOperator) activateChannel(object *pool.PooledObject) error {
	channelWrap, ok := object.Object.(*ChannelWrap)
	if !ok || channelWrap == nil {
		return errors.New("get channel error: nil value")
	}

	// 如果通道有效，直接返回
	if channelWrap.channel != nil && !channelWrap.channel.IsClosed() {
		return nil
	}

	// 否则重新检查并重建通道
	return r.recheck(channelWrap)
}

// connect 建立 RabbitMQ 连接
func (r *RabbitMQOperator) connect(ctx context.Context) (err error) {
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	// 重试连接
	for i := 0; i < RetryTimes; i++ {
		r.client.conn, err = amqp.DialTLS(r.config.addr(), &tls.Config{InsecureSkipVerify: true})
		if err == nil {
			break
		}
		r.l.Warn(ctx, "connect failed, retrying...",
			"error", err.Error(), "attempt", i+1)
		<-ticker.C
	}

	if err != nil {
		return fmt.Errorf("failed to connect after %d attempts: %w", RetryTimes, err)
	}

	// 设置连接关闭通知
	r.client.closeConnNotify = make(chan *amqp.Error, 1)
	r.client.conn.NotifyClose(r.client.closeConnNotify)
	r.isReady = true

	r.l.Info(ctx, "rabbitmq connected successfully", "addr", r.config.addr())
	return
}

// tryReConnect 尝试重连 - 修复空指针和连接管理问题
func (r *RabbitMQOperator) tryReConnect(daemon bool) (connected bool, err error) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	var count int
	ctx := context.Background()

	for {
		// 检查连接状态，必要时进行重连
		if !r.isReady && atomic.LoadInt32(&r.closed) != Closed {
			r.client.connMu.Lock()

			// 双重检查，避免重复连接
			if r.isReady && atomic.LoadInt32(&r.closed) != Closed {
				r.client.connMu.Unlock()
				continue
			}

			// 尝试建立新连接
			r.client.conn, err = amqp.DialTLS(r.config.addr(), &tls.Config{InsecureSkipVerify: true})
			if err == nil {
				r.client.closeConnNotify = make(chan *amqp.Error, 1)
				r.client.conn.NotifyClose(r.client.closeConnNotify)
				r.isReady = true
				r.client.connMu.Unlock()
				r.l.Info(ctx, fmt.Sprintf("rabbitmq connect success[addr:%s]", r.config.addr()))
				continue
			}

			r.client.connMu.Unlock()

			// 非守护模式下的重试逻辑
			if !daemon {
				count++
				if count >= RetryTimes {
					return false, err
				}
			}

			select {
			case s := <-sig:
				r.l.Info(ctx, fmt.Sprintf("received signal：[%v], exiting... ", s))
				return false, fmt.Errorf("connect fail, received signal：[%v]", s)
			case <-r.closeCh:
				r.l.Info(ctx, "rabbitmq is closed, exiting...")
				return false, errors.New("connect fail, rabbitmq is closed")
			case <-ticker.C:
			}
		}

		// 非守护模式立即返回
		if !daemon {
			connected = true
			break
		}

		// 守护模式下的信号处理
		select {
		case s := <-sig:
			r.l.Info(ctx, fmt.Sprintf("received signal：[%v], exiting daemon program... ", s))
			return false, fmt.Errorf("received signal：[%v], exiting daemon program", s)
		case <-r.closeCh:
			r.l.Info(ctx, "rabbitmq is closed, exiting daemon program...")
			return false, errors.New("rabbitmq is closed, exiting daemon program")
		case <-r.client.closeConnNotify:
			r.isReady = false
			r.l.Error(ctx, "rabbitmq disconnects unexpectedly, retrying...")
		}
	}
	return connected, nil
}

// confirmOne 确认单条消息
func (r *RabbitMQOperator) confirmOne(ctx context.Context, confirms <-chan amqp.Confirmation) (ok bool) {
	// 添加超时机制，避免永久阻塞
	confirmTimeout := time.NewTimer(5 * time.Second)
	defer confirmTimeout.Stop()

	select {
	case <-r.closeCh:
		r.l.Info(ctx, "rabbitmq is closed, confirm canceled, exiting...")
		return false
	case <-confirmTimeout.C:
		r.l.Warn(ctx, "confirm timeout, message may not be confirmed")
		return false
	case confirmed, ok := <-confirms:
		if !ok {
			r.l.Warn(ctx, "confirm channel closed")
			return false
		}
		if confirmed.DeliveryTag == 0 {
			// delivery tag 为 0 通常表示通道问题
			r.l.Warn(ctx, "received confirmation with delivery tag 0, channel may be in bad state")
			return false
		}
		if confirmed.Ack {
			r.l.Debug(ctx, "message confirmed successfully", "delivery_tag", confirmed.DeliveryTag)
			return true
		} else {
			r.l.Warn(ctx, "message not acknowledged", "delivery_tag", confirmed.DeliveryTag)
			return false
		}
	}
}

// SetLogger 设置日志记录器
func (r *RabbitMQOperator) SetLogger(logger amqp.Logging) {
	amqp.Logger = logger
}

// recheck 重新检查通道状态，必要时重建
func (r *RabbitMQOperator) recheck(c *ChannelWrap) (err error) {
	if c == nil {
		return errors.New("nil channelWrap")
	}

	// 如果通道无效，重建通道
	if c.channel == nil || c.channel.IsClosed() {
		if _, err = r.tryReConnect(false); err != nil {
			return err
		}

		c.channel, err = r.client.conn.Channel()
		if err != nil {
			return err
		}

		// 重新设置通道通知
		if c.isConsumer {
			c.closeChan = make(chan *amqp.Error)
			c.channel.NotifyClose(c.closeChan)
			c.openConfirm = false
		} else {
			c.closeChan = nil
		}

		if c.openConfirm {
			c.confirmChan = make(chan amqp.Confirmation, 20)
			c.channel.NotifyPublish(c.confirmChan)
		}
	} else {
		// 检查通道状态一致性
		if !c.isConsumer && c.closeChan != nil {
			c.channel = nil
			return r.recheck(c)
		}

		if c.isConsumer && c.closeChan == nil {
			c.closeChan = make(chan *amqp.Error)
			c.channel.NotifyClose(c.closeChan)
			c.openConfirm = false
		}

		if !c.openConfirm && c.confirmChan != nil {
			c.channel = nil
			return r.recheck(c)
		}

		if c.openConfirm && c.confirmChan == nil {
			c.confirmChan = make(chan amqp.Confirmation, 20)
			c.channel.NotifyPublish(c.confirmChan)
		}
	}
	return nil
}

// BorrowChannel 从池中借用通道
func (r *RabbitMQOperator) BorrowChannel(ctx context.Context, openConfirm bool) (channelWrap *ChannelWrap, err error) {
	var object any
	if openConfirm {
		object, err = r.client.confirmChanPool.BorrowObject(ctx)
	} else {
		object, err = r.client.chanPool.BorrowObject(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("borrow channel failed: %w", err)
	}

	channelWrap, ok := object.(*ChannelWrap)
	if !ok {
		return nil, errors.New("invalid channel type from pool")
	}
	return channelWrap, nil
}

// PushChannel 将通道归还到池中
func (r *RabbitMQOperator) PushChannel(ctx context.Context, channelWrap *ChannelWrap) (err error) {
	if channelWrap == nil {
		return errors.New("channelWrap is nil")
	}

	if channelWrap.channel != nil && channelWrap.channel.IsClosed() {
		// 通道已关闭，不归还到池中
		return nil
	}

	if channelWrap.openConfirm {
		err = r.client.confirmChanPool.ReturnObject(ctx, channelWrap)
	} else {
		err = r.client.chanPool.ReturnObject(ctx, channelWrap)
	}
	return
}

// PushDelayMessage 推送延迟消息
func (r *RabbitMQOperator) PushDelayMessage(ctx context.Context, body *PushDelayBody, opts ...OptionFunc) (err error) {
	return r.pushWithRetry(ctx, body, r.pushDelayMessageCore, opts...)
}

// pushDelayMessageCore 推送延迟消息的核心实现
func (r *RabbitMQOperator) pushDelayMessageCore(ctx context.Context, bodyInterface PushBodyInterface, opts ...OptionFunc) (err error) {
	body := bodyInterface.(*PushDelayBody)

	// 验证延迟时间参数
	if body.DelayTime == 0 && body.Expiration == "" {
		return errors.New("no expire time set")
	}

	// 借用通道
	channelWrap, err := r.BorrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return fmt.Errorf("borrow channel failed: %w", err)
	}
	defer func() {
		if returnErr := r.PushChannel(ctx, channelWrap); returnErr != nil {
			r.l.Warn(ctx, "return channel to pool failed", "err", returnErr.Error())
		}
	}()

	// 计算延迟时间并构建延迟队列名称
	delayStr := strconv.FormatInt(body.DelayTime.Milliseconds(), 10)
	delayQueue := "potato_delay_queue:" + body.ExchangeName

	// 设置默认交换机类型
	if string(body.ExchangeType) == "" {
		body.ExchangeType = Fanout
	}

	// 获取队列锁，确保同一队列的声明操作是串行的
	queueLock := r.queueLocks.GetLock(delayQueue)
	queueLock.Lock()
	defer queueLock.Unlock()

	// 声明交换机
	if err = r.DeclareExchange(channelWrap, body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...); err != nil {
		return fmt.Errorf("declare exchange failed: %w", err)
	}

	// 声明延迟队列（死信队列）
	if _, err = r.DeclareQueue(channelWrap, delayQueue, amqp.Table{
		"x-dead-letter-exchange":    body.ExchangeName,
		"x-dead-letter-routing-key": body.RoutingKey,
	}); err != nil {
		return fmt.Errorf("declare delay queue failed: %w", err)
	}

	// 处理交换机到目标队列的绑定
	for queue, bindingKey := range body.BindingKeyMap {
		table := body.QueueArgs[queue]

		// 获取目标队列的锁
		targetQueueLock := r.queueLocks.GetLock(queue)
		targetQueueLock.Lock()

		// 声明目标队列
		if _, err = r.DeclareQueue(channelWrap, queue, table, opts...); err != nil {
			targetQueueLock.Unlock()
			return fmt.Errorf("declare target queue failed: %w", err)
		}

		// 绑定队列到交换机
		if err = r.BindQueue(channelWrap, body.ExchangeName, bindingKey, queue, nil, opts...); err != nil {
			targetQueueLock.Unlock()
			return fmt.Errorf("bind queue failed: %w", err)
		}
		targetQueueLock.Unlock()
	}

	// 创建发布消息
	publishingMsg := r.createPublishingMessage(&body.Publishing)
	if body.Expiration == "" {
		publishingMsg.Expiration = delayStr
	}

	// 发送消息到延迟队列
	if err = channelWrap.channel.PublishWithContext(ctx, "", delayQueue, false, false, publishingMsg); err != nil {
		return fmt.Errorf("publish message failed: %w", err)
	}

	// 确认模式下的消息确认
	if body.OpenConfirm && channelWrap.openConfirm {
		if confirmed := r.confirmOne(ctx, channelWrap.confirmChan); !confirmed {
			return errors.New("push not confirmed")
		}
	}
	return
}

// PushExchange 向交换机推送消息
func (r *RabbitMQOperator) PushExchange(ctx context.Context, body *ExchangePushBody, opts ...OptionFunc) (err error) {
	return r.pushWithRetry(ctx, body, r.pushExchangeCore, opts...)
}

// PushQueue 向队列推送消息
func (r *RabbitMQOperator) PushQueue(ctx context.Context, body *QueuePushBody, opts ...OptionFunc) (err error) {
	return r.pushWithRetry(ctx, body, r.pushQueueCore, opts...)
}

// Push 通用推送方法
func (r *RabbitMQOperator) Push(ctx context.Context, body *PushBody, opts ...OptionFunc) (err error) {
	return r.pushWithRetry(ctx, body, r.pushCore, opts...)
}

// pushWithRetry 带重试的推送方法 - 修复日志格式问题
func (r *RabbitMQOperator) pushWithRetry(ctx context.Context, body PushBodyInterface, pushFunc func(context.Context, PushBodyInterface, ...OptionFunc) error, opts ...OptionFunc) (err error) {
	defer handlePanic(ctx, r)

	// 生成消息ID
	if body.GetMessageId() == "" {
		body.SetMessageId(strings.ReplaceAll(uuid.NewString(), "-", ""))
	}

	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	// 重试逻辑
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&r.closed) == Closed {
			r.l.Error(ctx, "rabbitmq connection is closed, push cancel", "msg_id", body.GetMessageId())
			return errors.New("connection is closed")
		}

		// 执行推送
		if pushErr := pushFunc(ctx, body, opts...); pushErr != nil {
			r.l.Warn(ctx, fmt.Sprintf("[push] Push failed after %f seconds, retrying...", DefaultRetryWaitTimes.Seconds()),
				"err", pushErr.Error(), "msg_id", body.GetMessageId(), "retry_count", i+1)

			select {
			case <-r.closeCh:
				r.l.Error(ctx, "[push] rmq closed, push msg cancel", "msg_id", body.GetMessageId())
				return errors.New("rabbitmq connection closed")
			case <-ticker.C:
				// 继续重试
			}
			continue
		}

		r.l.Info(ctx, "[push] Push msg success", "msg_id", body.GetMessageId())
		return nil
	}

	r.l.Info(ctx, fmt.Sprintf("[push] Push msg failed. msg ---> %s", string(body.GetBody())),
		"msg_id", body.GetMessageId())
	return fmt.Errorf("push failed after %d retries", RetryTimes)
}

// createPublishingMessage 创建发布消息的通用方法
func (r *RabbitMQOperator) createPublishingMessage(publishing *amqp.Publishing) amqp.Publishing {
	msg := amqp.Publishing{
		Timestamp:       times.Now(),
		Body:            publishing.Body,
		Headers:         publishing.Headers,
		ContentEncoding: publishing.ContentEncoding,
		ReplyTo:         publishing.ReplyTo,
		Expiration:      publishing.Expiration,
		MessageId:       publishing.MessageId,
		Type:            publishing.Type,
		UserId:          publishing.UserId,
		AppId:           publishing.AppId,
		Priority:        publishing.Priority,
		DeliveryMode:    publishing.DeliveryMode,
		ContentType:     publishing.ContentType,
	}

	// 设置默认值
	if msg.DeliveryMode == 0 {
		msg.DeliveryMode = amqp.Persistent
	}
	if msg.ContentType == "" {
		msg.ContentType = "text/plain"
	}

	return msg
}

// pushQueueCore 向队列推送消息的核心实现
func (r *RabbitMQOperator) pushQueueCore(ctx context.Context, bodyInterface PushBodyInterface, opts ...OptionFunc) (err error) {
	body := bodyInterface.(*QueuePushBody)

	channelWrap, err := r.BorrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return fmt.Errorf("borrow channel failed: %w", err)
	}
	defer func() {
		if returnErr := r.PushChannel(ctx, channelWrap); returnErr != nil {
			r.l.Warn(ctx, "return channel to pool failed", "err", returnErr.Error())
		}
	}()

	// 获取队列锁，确保同一队列的操作安全
	queueLock := r.queueLocks.GetLock(body.QueueName)
	queueLock.Lock()
	defer queueLock.Unlock()

	// 声明队列
	if _, err = r.DeclareQueue(channelWrap, body.QueueName, body.Args, opts...); err != nil {
		return fmt.Errorf("declare queue failed: %w", err)
	}

	// 发送消息
	publishingMsg := r.createPublishingMessage(&body.Publishing)
	if err = channelWrap.channel.PublishWithContext(ctx, "", body.QueueName, false, false, publishingMsg); err != nil {
		return fmt.Errorf("publish message failed: %w", err)
	}

	// 确认模式处理
	if body.OpenConfirm && channelWrap.openConfirm {
		// 使用改进的确认逻辑
		if confirmed := r.confirmOne(ctx, channelWrap.confirmChan); !confirmed {
			return errors.New("push not confirmed")
		}
	}
	return
}

// pushExchangeCore 向交换机推送消息的核心实现
func (r *RabbitMQOperator) pushExchangeCore(ctx context.Context, bodyInterface PushBodyInterface, opts ...OptionFunc) (err error) {
	body := bodyInterface.(*ExchangePushBody)

	channelWrap, err := r.BorrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return fmt.Errorf("borrow channel failed: %w", err)
	}
	defer func() {
		if returnErr := r.PushChannel(ctx, channelWrap); returnErr != nil {
			r.l.Warn(ctx, "return channel to pool failed", "err", returnErr.Error())
		}
	}()

	// 设置默认交换机类型
	if body.ExchangeType == "" {
		body.ExchangeType = Fanout
	}

	if len(body.QueueArgs) == 0 {
		body.QueueArgs = make(map[string]amqp.Table)
	}

	// 声明交换机
	if err = r.DeclareExchange(channelWrap, body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...); err != nil {
		return fmt.Errorf("declare exchange failed: %w", err)
	}

	// 处理交换机到队列的绑定
	for queue, bindingKey := range body.BindingKeyMap {
		table := body.QueueArgs[queue]

		// 获取队列锁
		queueLock := r.queueLocks.GetLock(queue)
		queueLock.Lock()

		// 声明队列
		if _, err = r.DeclareQueue(channelWrap, queue, table, opts...); err != nil {
			queueLock.Unlock()
			return fmt.Errorf("declare queue failed: %w", err)
		}

		// 绑定队列到交换机
		if err = r.BindQueue(channelWrap, body.ExchangeName, bindingKey, queue, nil, opts...); err != nil {
			queueLock.Unlock()
			return fmt.Errorf("bind queue failed: %w", err)
		}
		queueLock.Unlock()
	}

	// 发送消息
	publishingMsg := r.createPublishingMessage(&body.Publishing)
	if err = channelWrap.channel.PublishWithContext(ctx, body.ExchangeName, body.RoutingKey, false, false, publishingMsg); err != nil {
		return fmt.Errorf("publish message failed: %w", err)
	}

	// 确认模式处理
	if body.OpenConfirm && channelWrap.openConfirm {
		if confirmed := r.confirmOne(ctx, channelWrap.confirmChan); !confirmed {
			return errors.New("push confirmed fail")
		}
	}
	return
}

// pushCore 通用推送核心实现
func (r *RabbitMQOperator) pushCore(ctx context.Context, bodyInterface PushBodyInterface, opts ...OptionFunc) (err error) {
	body := bodyInterface.(*PushBody)

	channelWrap, err := r.BorrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return fmt.Errorf("borrow channel failed: %w", err)
	}
	defer func() {
		if returnErr := r.PushChannel(ctx, channelWrap); returnErr != nil {
			r.l.Warn(ctx, "return channel to pool failed", "err", returnErr.Error())
		}
	}()

	// 交换机模式处理
	if body.ExchangeName != "" {
		if err = r.DeclareExchange(channelWrap, body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...); err != nil {
			r.l.Error(ctx, "MQ failed to declare the exchange", "err", err.Error(), "msg_id", body.MessageId)
			return fmt.Errorf("declare exchange failed: %w", err)
		}

		// 处理队列绑定
		for queue, bindingKey := range body.BindingKeyMap {
			table := body.QueueArgs[queue]

			// 获取队列锁
			queueLock := r.queueLocks.GetLock(queue)
			queueLock.Lock()

			if _, err = r.DeclareQueue(channelWrap, queue, table, opts...); err != nil {
				queueLock.Unlock()
				r.l.Error(ctx, "MQ declare queue failed", "err", err.Error(), "msg_id", body.MessageId)
				return fmt.Errorf("declare queue failed: %w", err)
			}

			if err = r.BindQueue(channelWrap, body.ExchangeName, bindingKey, queue, nil, opts...); err != nil {
				queueLock.Unlock()
				r.l.Error(ctx, "MQ binding queue failed", "err", err.Error(), "msg_id", body.MessageId)
				return fmt.Errorf("bind queue failed: %w", err)
			}
			queueLock.Unlock()
		}
	}

	// 队列模式处理
	if body.QueueName != "" {
		// 获取队列锁
		queueLock := r.queueLocks.GetLock(body.QueueName)
		queueLock.Lock()
		defer queueLock.Unlock()

		if _, err = r.DeclareQueue(channelWrap, body.QueueName, body.Args, opts...); err != nil {
			r.l.Error(ctx, "MQ declare queue failed", "err", err.Error(), "msg_id", body.MessageId)
			return fmt.Errorf("declare queue failed: %w", err)
		}
	}

	// 发送消息
	publishingMsg := r.createPublishingMessage(&body.Publishing)
	if err = channelWrap.channel.PublishWithContext(ctx, body.ExchangeName, body.RoutingKey, false, false, publishingMsg); err != nil {
		r.l.Error(ctx, "MQ task failed to be sent", "err", err.Error(), "msg_id", body.MessageId)
		return fmt.Errorf("publish message failed: %w", err)
	}

	// 确认模式处理
	if body.OpenConfirm && channelWrap.openConfirm {
		if confirmed := r.confirmOne(ctx, channelWrap.confirmChan); !confirmed {
			return errors.New("push confirmed fail")
		}
	}
	return
}

// Consume 消费消息 - 使用队列锁确保同一队列的消费操作安全
func (r *RabbitMQOperator) Consume(ctx context.Context, param *ConsumeBody) (<-chan amqp.Delivery, error) {
	// 获取队列锁，确保同一队列的消费操作串行化
	queueLock := r.queueLocks.GetLock(param.QueueName)
	queueLock.Lock()
	defer queueLock.Unlock()

	var channelWrap *ChannelWrap
	resChan, consumerTag, err := r.consumeCore(ctx, param, &channelWrap)
	if err != nil {
		return nil, err
	}

	contents := make(chan amqp.Delivery, 5)

	go func() {
		ctxBack := context.Background()
		handlePanic(ctxBack, r)

		valueCh, exist := r.client.cancelChan.Load(param.QueueName)
		if !exist {
			r.client.cacheMu.Lock()
			valueCh, exist = r.client.cancelChan.Load(param.QueueName)

			if !exist {
				valueCh = make(chan bool)
				r.client.cancelChan.Store(param.QueueName, valueCh)
			}
			r.client.cacheMu.Unlock()
		}

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

		ticker := time.NewTicker(DefaultRetryWaitTimes)
		defer ticker.Stop()
		var innerErr error
		var innerConsume bool

	process:
		if innerConsume {
			resChan, consumerTag, innerErr = r.consumeCore(ctxBack, param, &channelWrap)
			// 消费失败时重试
			for innerErr != nil {
				r.l.Error(ctxBack, fmt.Sprintf("[consume:%s] consume msg error, retrying ...", param.QueueName), "err", innerErr.Error())
				select {
				case s := <-sig:
					r.l.Info(ctx, fmt.Sprintf("[consume:%s] recived： %v, exiting... ", param.QueueName, s))
					return
				case <-r.closeCh:
					r.l.Info(ctx, fmt.Sprintf("[consume:%s] rabbitmq is closed, exiting...", param.QueueName))
					return
				case <-valueCh.(chan bool):
					r.l.Info(ctx, fmt.Sprintf("[consume:%s] consume cancel, exiting...", param.QueueName))
					_ = channelWrap.Close()
					return
				case <-ticker.C:
				}
				resChan, consumerTag, innerErr = r.consumeCore(ctxBack, param, &channelWrap)
			}
		} else {
			innerConsume = true
		}

		// 循环消费数据
		for {
			select {
			case s := <-sig:
				r.l.Info(ctx, fmt.Sprintf("[consume:%s] recived： %v, exiting... ", param.QueueName, s))
				return
			case <-r.closeCh:
				r.l.Info(ctx, fmt.Sprintf("[consume:%s] rabbitmq is closed, exiting...", param.QueueName))
				return
			case <-valueCh.(chan bool):
				r.l.Info(ctx, fmt.Sprintf("[consume:%s] consume cancel, exiting...", param.QueueName))
				cancelErr := channelWrap.channel.Cancel(consumerTag, false)
				if cancelErr != nil {
					r.l.Error(ctx, fmt.Sprintf("[consume:%s] consume cancel error, exiting...", param.QueueName), "err", cancelErr.Error())
				}
				r.client.cancelChan.Delete(param.QueueName)
				_ = channelWrap.Close()
				return
			case <-channelWrap.closeChan:
				r.l.Warn(ctx, fmt.Sprintf("[consume:%s] rmq channel is closed, reconsume...", param.QueueName))
				goto process
			case item := <-resChan:
				if len(item.Body) == 0 {
					continue
				}
				select {
				case s := <-sig:
					r.l.Info(ctx, fmt.Sprintf("[consume:%s] recived： %v, exiting... ", param.QueueName, s))
					return
				case <-r.closeCh:
					r.l.Info(ctx, fmt.Sprintf("[consume:%s] rabbitmq is closed, exiting...", param.QueueName))
					return
				case <-channelWrap.closeChan:
					r.l.Warn(ctx, fmt.Sprintf("[consume:%s] rmq channel is closed, reconsume...", param.QueueName))
					goto process
				case contents <- item:
					r.l.Info(ctx, fmt.Sprintf("[consume:%s] recived msg success: msg_id --> %s", param.QueueName, item.MessageId))
				}
			case <-ticker.C:
				continue
			}
		}
	}()
	return contents, nil
}

// consumeCore 消费消息的核心实现
func (r *RabbitMQOperator) consumeCore(ctx context.Context, param *ConsumeBody, channelWrap **ChannelWrap,
	opts ...OptionFunc) (contents <-chan amqp.Delivery, consumerTag string, err error) {
	if !r.isReady {
		err = errors.New("connection is not ready")
		return
	}
	if *channelWrap != nil {
		_ = (*channelWrap).Close()
	}
	*channelWrap, err = r.BorrowChannel(ctx, false)
	if err != nil {
		return
	}
	(*channelWrap).isConsumer = true
	err = r.recheck(*channelWrap)
	if err != nil {
		return
	}

	// 设置预取计数
	if param.FetchCount == 0 {
		param.FetchCount = 1
	}

	// 声明队列
	_, err = r.DeclareQueue(*channelWrap, param.QueueName, param.QueueArgs, opts...)
	if err != nil {
		return
	}

	// 绑定交换机
	if param.ExchangeName != "" {
		err = r.BindQueue(*channelWrap, param.ExchangeName, param.RoutingKey, param.QueueName, nil, opts...)
		if err != nil {
			r.l.Error(ctx, "binding queue failed", "err", err.Error())
			return
		}
	}

	// 设置 QoS
	err = (*channelWrap).channel.Qos(param.FetchCount, 0, true)
	if err != nil {
		r.l.Error(ctx, "open qos error", "err", err.Error())
		return
	}

	// 设置消费参数
	var args amqp.Table
	if param.XPriority != 0 {
		args = amqp.Table{"x-priority": param.XPriority}
	}

	// 生成消费者标签并开始消费
	consumerTag = uniqueConsumerTag()
	contents, err = (*channelWrap).channel.Consume(param.QueueName, consumerTag, param.AutoAck, false, false, false, args)
	if err != nil {
		r.l.Error(ctx, "The acquisition of the consumption channel is abnormal", "err", err.Error())
		return
	}
	return
}

// Options 声明选项
type Options struct {
	durable    bool
	autoDelete bool
	exclusive  bool
	noWait     bool
	internal   bool
	ifUnUsed   bool
	ifEmpty    bool
}

type OptionFunc func(*Options)

func WithDeclareOptionDurable(durable bool) OptionFunc {
	return func(options *Options) {
		options.durable = durable
	}
}

func WithDeclareOptionAutoDelete(autoDelete bool) OptionFunc {
	return func(options *Options) {
		options.autoDelete = autoDelete
	}
}

func WithDeclareOptionExclusive(exclusive bool) OptionFunc {
	return func(options *Options) {
		options.exclusive = exclusive
	}
}

func WithDeclareOptionNoWait(noWait bool) OptionFunc {
	return func(options *Options) {
		options.noWait = noWait
	}
}

func WithDeclareOptionInternal(internal bool) OptionFunc {
	return func(options *Options) {
		options.internal = internal
	}
}

func WithDelOptionIfEmpty(ifEmpty bool) OptionFunc {
	return func(options *Options) {
		options.ifEmpty = ifEmpty
	}
}

func WithDelOptionIfUnused(ifUnUsed bool) OptionFunc {
	return func(options *Options) {
		options.ifUnUsed = ifUnUsed
	}
}

// DeclareExchange 声明交换机
func (r *RabbitMQOperator) DeclareExchange(channelWrap *ChannelWrap, exchangeName,
	exchangeType string, args amqp.Table, opts ...OptionFunc) (err error) {
	options := &Options{
		durable:    true,
		autoDelete: false,
		internal:   false,
		noWait:     false,
	}
	for _, opt := range opts {
		opt(options)
	}

	opKey := fmt.Sprintf("declare_exchange:%s", exchangeName)
	_, ok := r.opCache.Load(opKey)
	if ok {
		options.noWait = true
	} else {
		r.declareMu.Lock()
		defer r.declareMu.Unlock()
	}

	err = channelWrap.channel.ExchangeDeclarePassive(exchangeName, exchangeType, options.durable, options.autoDelete, options.internal, options.noWait, args)
	if err == nil {
		if !ok {
			r.opCache.Store(opKey, true)
		}
		return
	}

	err = r.recheck(channelWrap)
	if err != nil {
		return
	}
	err = channelWrap.channel.ExchangeDeclare(exchangeName, exchangeType, options.durable, options.autoDelete, options.internal, options.noWait, args)
	if err != nil {
		return
	}

	if !ok {
		r.opCache.Store(opKey, true)
	}
	return
}

// DeclareQueue 声明队列
func (r *RabbitMQOperator) DeclareQueue(channelWrap *ChannelWrap, queueName string,
	args amqp.Table, opts ...OptionFunc) (queue amqp.Queue, err error) {
	options := &Options{
		durable:    true,
		autoDelete: false,
		exclusive:  false,
		noWait:     false,
	}
	for _, opt := range opts {
		opt(options)
	}

	opKey := fmt.Sprintf("declare_queue:%s", queueName)
	_, ok := r.opCache.Load(opKey)
	if ok {
		options.noWait = true
	} else {
		r.declareMu.Lock()
		defer r.declareMu.Unlock()
	}

	queue, err = channelWrap.channel.QueueDeclarePassive(queueName, options.durable, options.autoDelete, options.exclusive, options.noWait, args)
	if err == nil {
		if !ok {
			r.opCache.Store(opKey, true)
		}
		return
	}

	err = r.recheck(channelWrap)
	if err != nil {
		return
	}
	queue, err = channelWrap.channel.QueueDeclare(queueName, options.durable, options.autoDelete, options.exclusive, options.noWait, args)
	if err != nil {
		return
	}

	if !ok {
		r.opCache.Store(opKey, true)
	}
	return
}

// CancelQueue 取消队列消费
func (r *RabbitMQOperator) CancelQueue(queueName string) (err error) {
	chanVal, ok := r.client.cancelChan.Load(queueName)
	if !ok {
		return
	}

	close(chanVal.(chan bool))
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; i < 5; i++ {
		_, ok = r.client.cancelChan.Load(queueName)
		if !ok {
			return
		}
		<-ticker.C
	}
	r.l.Warn(context.Background(), "cancel consumer timeout", "queue", queueName)
	return
}

// BindQueue 绑定队列到交换机
func (r *RabbitMQOperator) BindQueue(channelWrap *ChannelWrap, exchangeName, routingKey, queueName string, args amqp.Table, opts ...OptionFunc) (err error) {
	options := &Options{
		noWait: false,
	}
	opKey := fmt.Sprintf("bind_queue:%s_%s", exchangeName, queueName)
	_, ok := r.opCache.Load(opKey)
	if ok {
		options.noWait = true
	}

	for _, opt := range opts {
		opt(options)
	}
	err = channelWrap.channel.QueueBind(queueName, routingKey, exchangeName, options.noWait, args)
	if err != nil {
		return err
	}

	if !ok {
		r.opCache.Store(opKey, true)
	}
	return
}

// UnBindQueue 解绑队列
func (r *RabbitMQOperator) UnBindQueue(channelWrap *ChannelWrap, exchangeName, queueName, routingKey string, args amqp.Table) (err error) {
	err = channelWrap.channel.QueueUnbind(queueName, routingKey, exchangeName, args)
	if err != nil {
		return err
	}

	opKey := fmt.Sprintf("bind_queue:%s_%s", exchangeName, queueName)
	r.opCache.Delete(opKey)
	return
}

// DeleteExchange 删除交换机
func (r *RabbitMQOperator) DeleteExchange(channelWrap *ChannelWrap, exchangeName string, opts ...OptionFunc) (err error) {
	options := &Options{
		ifUnUsed: true,
		noWait:   false,
	}
	for _, opt := range opts {
		opt(options)
	}
	err = channelWrap.channel.ExchangeDelete(exchangeName, options.ifUnUsed, options.noWait)
	if err != nil {
		return err
	}
	r.opCache.Delete(fmt.Sprintf("declare_exchange:%s", exchangeName))
	return
}

// DeleteQueue 删除队列
func (r *RabbitMQOperator) DeleteQueue(channelWrap *ChannelWrap, queueName string, opts ...OptionFunc) (err error) {
	options := &Options{
		ifUnUsed: true,
		ifEmpty:  true,
		noWait:   false,
	}
	for _, opt := range opts {
		opt(options)
	}

	_, err = channelWrap.channel.QueueDelete(queueName, options.ifUnUsed, options.ifEmpty, options.noWait)
	if err != nil {
		return err
	}
	r.opCache.Delete(fmt.Sprintf("declare_queue:%s", queueName))
	return
}

// GetMessageCount 获取队列消息数量
func (r *RabbitMQOperator) GetMessageCount(channelWrap *ChannelWrap, queueName string) (count int, err error) {
	queue, err := channelWrap.channel.QueueInspect(queueName)
	if err != nil {
		return
	}
	count = queue.Messages
	return
}

// Close 关闭 RabbitMQ 操作器
func (r *RabbitMQOperator) Close(ctx context.Context) (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if atomic.LoadInt32(&r.closed) == Closed {
		return nil
	}
	// 关闭连接
	if r.client.conn != nil && !r.client.conn.IsClosed() {
		err = r.client.conn.Close()
		if err != nil {
			r.l.Error(ctx, "close rabbitmq connection failed", "err", err.Error())
		}
	}

	// 标记为已关闭
	atomic.StoreInt32(&r.closed, Closed)
	// 关闭通道池
	if r.client.chanPool != nil {
		r.client.chanPool.Close(ctx)
	}
	if r.client.confirmChanPool != nil {
		r.client.confirmChanPool.Close(ctx)
	}

	// 发送关闭信号
	select {
	case <-r.closeCh:
		// 已经关闭
	default:
		close(r.closeCh)
	}

	r.isReady = false
	r.l.Info(ctx, "rabbitmq operator closed successfully")
	return
}

// Ack 确认消息
// Ack 确认消息 - 简化版本，避免通道竞态条件
func (r *RabbitMQOperator) Ack(ctx context.Context, msg amqp.Delivery) {
	// 使用带缓冲的通道和上下文控制
	ctx, cancel := context.WithTimeout(ctx, DefaultRetryWaitTimes)
	defer cancel()

	resultCh := make(chan error, 1)

	go func() {
		resultCh <- msg.Ack(false)
	}()

	select {
	case <-r.closeCh:
		r.l.Info(ctx, "[Consume] mq closed, ack cancel", "routing_key", msg.RoutingKey, "msg_id", msg.MessageId)
		return
	case err := <-resultCh:
		if err != nil {
			r.l.Info(ctx, "[Ack] ack failed", "msg_id", msg.MessageId, "error", err.Error())
		} else {
			r.l.Info(ctx, "[Ack] ack done", "msg_id", msg.MessageId)
		}
	case <-ctx.Done():
		r.l.Info(ctx, "[Ack] ack timeout", "msg_id", msg.MessageId)
	}
}

// Nack 拒绝消息
// Nack 拒绝消息 - 简化版本
func (r *RabbitMQOperator) Nack(ctx context.Context, msg amqp.Delivery) {
	ctx, cancel := context.WithTimeout(ctx, DefaultRetryWaitTimes)
	defer cancel()

	resultCh := make(chan error, 1)

	go func() {
		resultCh <- msg.Nack(false, true)
	}()

	select {
	case <-r.closeCh:
		r.l.Info(ctx, "[Consume] mq closed, nack cancel", "routing_key", msg.RoutingKey, "msg_id", msg.MessageId)
		return
	case err := <-resultCh:
		if err != nil {
			r.l.Info(ctx, "[Nack] nack failed", "msg_id", msg.MessageId, "error", err.Error())
		} else {
			r.l.Info(ctx, "[Nack] nack done", "msg_id", msg.MessageId)
		}
	case <-ctx.Done():
		r.l.Info(ctx, "[Nack] nack timeout", "msg_id", msg.MessageId)
	}
}
