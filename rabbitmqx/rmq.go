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
			zapx.GetLogger().WithError(err).Error("copy rmq config error, skipping ...")
			return
		}
		err = InitRabbitMQOperator(mqConf)
		if err != nil {
			zapx.GetLogger().WithError(err).Error("init rmq Client error, skipping ...")
		}
	}
}

func genUUID() string {
	return uuid.NewString()
}

// InitRabbitMQOperator 负责初始化全局变量operator，NewRabbitMQOperator函数负责根据配置创建rmq客户端对象供外部调用
func InitRabbitMQOperator(config *MQConfig) (err error) {
	operator, err = NewRabbitMQOperator(config)
	if err != nil {
		return
	}
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

type ChannelWrap struct {
	channel     *amqp.Channel
	openConfirm bool                   // 当isConsumer为false，即推送channel是否打开确认模式
	isConsumer  bool                   // 是否为消费队列使用
	confirmChan chan amqp.Confirmation // 用于推送channel的消息推送确认
	closeChan   chan *amqp.Error       // 用于消费channel的监听
}

func (c *ChannelWrap) Close() error {
	if c.channel == nil || c.channel.IsClosed() {
		return nil
	}
	return c.channel.Close()
}

func (c *MQConfig) Validate() error {
	if c.Host == "" {
		return errors.New("host is empty")
	}
	if c.Port == 0 {
		return errors.New("port is empty")
	}
	return nil
}

func (c *MQConfig) addr() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", c.Username, c.Password, c.Host, c.Port)
}

type RabbitMQOperator struct {
	name      string
	config    *MQConfig
	client    *Client
	opCache   sync.Map
	closeCh   chan bool
	isReady   bool
	closed    int32
	mu        sync.Mutex
	declareMu sync.Mutex
	l         log.Logger
}

type Client struct {
	conn            *amqp.Connection
	chanPool        *pool.ObjectPool // 用于关闭Publisher Confirms 模式
	confirmChanPool *pool.ObjectPool // 用于开启Publisher Confirms 模式
	cancelChan      sync.Map
	cacheMu         sync.Mutex
	connMu          sync.Mutex
	closeConnNotify chan *amqp.Error
}

// NewRabbitMQOperator 该函数负责根据配置创建rmq客户端对象供外部调用
func NewRabbitMQOperator(config *MQConfig, opts ...ConnOption) (op *RabbitMQOperator, err error) {
	// validate config param
	if err = config.Validate(); err != nil {
		return
	}
	ctx := context.Background()

	defaultConfig := DefaultConfig()
	for _, opt := range opts {
		opt(defaultConfig)
	}
	op = &RabbitMQOperator{
		config: config,
		client: &Client{
			closeConnNotify: make(chan *amqp.Error, 1),
		},
		closeCh: make(chan bool),
		name:    genUUID(),
		l:       defaultConfig.l,
	}

	// init, connection info && logger
	op.init(ctx, &defaultConfig.ObjectPoolConfig)

	err = op.connect()
	if err != nil {
		return
	}
	// a new goroutine for check disconnect
	go func() {
		_, connectErr := op.tryReConnect(true)
		if connectErr != nil {
			op.l.Warn("daemon progress of reconnection has exited.", "error", err.Error())
		}
	}()

	return
}

func (r *RabbitMQOperator) init(ctx context.Context, config *pool.ObjectPoolConfig) {
	factory := pool.NewPooledObjectFactory(
		func(ctx context.Context) (interface{}, error) {
			if !r.isReady && atomic.LoadInt32(&r.closed) != Closed {
				connected, connectErr := r.tryReConnect(false)
				if !connected {
					return nil, connectErr
				}
			}
			channel, getChannelErr := r.client.conn.Channel()
			if getChannelErr != nil {
				return nil, getChannelErr
			}
			channelWrap := &ChannelWrap{
				channel:   channel,
				closeChan: make(chan *amqp.Error),
			}
			return channelWrap, nil
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			channelWrap, ok := object.Object.(*ChannelWrap)
			if !ok || channelWrap == nil {
				return nil
			}
			return channelWrap.Close()
		},
		func(ctx context.Context, object *pool.PooledObject) bool {
			channelWrap, ok := object.Object.(*ChannelWrap)
			if !ok || channelWrap == nil {
				return false
			}
			return channelWrap.channel != nil && !channelWrap.channel.IsClosed()
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			channelWrap, ok := object.Object.(*ChannelWrap)
			if !ok || channelWrap == nil {
				return errors.New("get channel error: nil value")
			}
			if channelWrap.channel != nil && !channelWrap.channel.IsClosed() {
				return nil
			}
			if !r.isReady && atomic.LoadInt32(&r.closed) != Closed {
				connected, connectErr := r.tryReConnect(false)
				if !connected {
					return connectErr
				}
			}
			channel, err := r.client.conn.Channel()
			if err != nil {
				return err
			}
			channelWrap.channel = channel
			if channelWrap.openConfirm {
				channelWrap.confirmChan = make(chan amqp.Confirmation, 1)
				channelWrap.channel.NotifyPublish(channelWrap.confirmChan)
			}
			return nil
		},
		nil,
	)
	confirmFactory := pool.NewPooledObjectFactory(
		func(ctx context.Context) (interface{}, error) {
			if !r.isReady && atomic.LoadInt32(&r.closed) != Closed {
				connected, connectErr := r.tryReConnect(false)
				if !connected {
					return nil, connectErr
				}
			}
			channel, getChannelErr := r.client.conn.Channel()
			if getChannelErr != nil {
				return nil, getChannelErr
			}
			err := channel.Confirm(false)
			if err != nil {
				return nil, err
			}
			channelWrap := &ChannelWrap{
				channel:     channel,
				openConfirm: true,
				confirmChan: make(chan amqp.Confirmation, 1),
			}
			channel.NotifyPublish(channelWrap.confirmChan)
			return channelWrap, nil
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			channelWrap, ok := object.Object.(*ChannelWrap)
			if !ok || channelWrap == nil {
				return nil
			}
			return channelWrap.channel.Close()
		},
		func(ctx context.Context, object *pool.PooledObject) bool {
			channelWrap, ok := object.Object.(*ChannelWrap)
			if !ok || channelWrap == nil {
				return false
			}
			return channelWrap.channel != nil && !channelWrap.channel.IsClosed()
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			channelWrap, ok := object.Object.(*ChannelWrap)
			if !ok || channelWrap == nil {
				return errors.New("get channel error: nil value")
			}
			if channelWrap.channel != nil && !channelWrap.channel.IsClosed() {
				return nil
			}
			if !r.isReady && atomic.LoadInt32(&r.closed) != Closed {
				connected, connectErr := r.tryReConnect(false)
				if !connected {
					return connectErr
				}
			}
			channel, err := r.client.conn.Channel()
			if err != nil {
				return err
			}
			channelWrap.channel = channel
			if channelWrap.openConfirm {
				channelWrap.confirmChan = make(chan amqp.Confirmation, 1)
				channelWrap.channel.NotifyPublish(channelWrap.confirmChan)
			}
			return nil
		},
		nil,
	)
	r.client.confirmChanPool = pool.NewObjectPool(ctx, confirmFactory, config)
	r.client.chanPool = pool.NewObjectPool(ctx, factory, config)
	return
}

func (r *RabbitMQOperator) connect() (err error) {
	// ready connect rmq
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()
	for i := 0; i < RetryTimes; i++ {
		r.client.conn, err = amqp.DialTLS(r.config.addr(), &tls.Config{InsecureSkipVerify: true})
		if err == nil {
			break
		}
		<-ticker.C
		r.l.Warn(fmt.Sprintf("wait %f seconds for retry to connect...", DefaultRetryWaitTimes.Seconds()))
	}

	if err != nil {
		return
	}
	r.client.closeConnNotify = make(chan *amqp.Error, 1)
	r.client.conn.NotifyClose(r.client.closeConnNotify)
	r.isReady = true
	return
}

func (r *RabbitMQOperator) tryReConnect(daemon bool) (connected bool, err error) {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()
	var count int
	for {
		//var err error
		if !r.isReady && atomic.LoadInt32(&r.closed) != Closed {
			r.client.connMu.Lock() // 加锁
			if r.isReady && atomic.LoadInt32(&r.closed) != Closed {
				r.client.connMu.Unlock() // 释放锁
				continue
			}
			r.client.conn, err = amqp.DialTLS(r.config.addr(), &tls.Config{InsecureSkipVerify: true})
			if err == nil {
				r.client.closeConnNotify = make(chan *amqp.Error, 1)
				r.client.conn.NotifyClose(r.client.closeConnNotify)
				r.isReady = true
				r.client.connMu.Unlock() // 释放锁
				r.l.Info("rabbitmq connect success[addr:%s]", r.config.addr())
				continue
			}

			r.client.connMu.Unlock() // 释放锁

			if !daemon {
				count++
				if count >= RetryTimes {
					return false, err
				}
			}

			select {
			case s := <-sig:
				r.l.Infof("received signal：[%v], exiting... ", s)
				return false, errors.New(fmt.Sprintf("connect fail, received signal：[%v]", s))
			case <-r.closeCh:
				r.l.Info("rabbitmq is closed, exiting...")
				return false, errors.New("connect fail, rabbitmq is closed")
			case <-ticker.C:
			}
		}
		if !daemon {
			connected = true
			break
		}

		select {
		case s := <-sig:
			r.l.Info("received signal：[%v], exiting daemon program... ", s)
			return false, errors.New(fmt.Sprintf("received signal：[%v], exiting daemon program", s))
		case <-r.closeCh:
			r.l.Info("rabbitmq is closed, exiting daemon program...")
			return false, errors.New("rabbitmq is closed, exiting daemon program")
		case <-r.client.closeConnNotify:
			r.isReady = false
			//r.client.channelCache = sync.Map{}
			//r.client.chCloseListener = sync.Map{}
			r.l.Error("rabbitmq disconnects unexpectedly, retrying...")
		}
	}
	return connected, nil
}

// 消息确认
func (r *RabbitMQOperator) confirmOne(confirms <-chan amqp.Confirmation) (ok bool) {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

	select {
	case s := <-sig:
		r.l.Info(fmt.Sprintf("recived： %v, exiting... ", s))
		return
	case <-r.closeCh:
		r.l.Info("rabbitmq is closed, confirm canceled, exiting...")
		return
	case confirmed := <-confirms:
		if confirmed.Ack {
			ok = true
		} else {
			r.l.Warn(fmt.Sprintf("confirmed delivery false of delivery tag: %d", confirmed.DeliveryTag))
		}
	}
	return
}

func (r *RabbitMQOperator) SetLogger(logger amqp.Logging) {
	amqp.Logger = logger
}

func (r *RabbitMQOperator) recheck(c *ChannelWrap) (err error) {
	if c == nil {
		return errors.New("nil channelWrap")
	}

	if c.channel == nil || c.channel.IsClosed() {
		_, err = r.tryReConnect(false)
		if err != nil {
			return err
		}
		c.channel, err = r.client.conn.Channel()

		if c.isConsumer {
			c.closeChan = make(chan *amqp.Error)
			c.channel.NotifyClose(c.closeChan)
			c.openConfirm = false
		} else {
			c.closeChan = nil
		}

		if c.openConfirm {
			c.confirmChan = make(chan amqp.Confirmation, 1)
			c.channel.NotifyPublish(c.confirmChan)
		}
	} else {
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
			c.confirmChan = make(chan amqp.Confirmation, 1)
			c.channel.NotifyPublish(c.confirmChan)
		}
	}
	return nil
}

func (r *RabbitMQOperator) borrowChannel(ctx context.Context, openConfirm bool) (channelWrap *ChannelWrap, err error) {
	var object any
	if openConfirm {
		object, err = r.client.confirmChanPool.BorrowObject(ctx)
	} else {
		object, err = r.client.chanPool.BorrowObject(ctx)
	}
	if err != nil {
		return
	}
	return object.(*ChannelWrap), nil
}

func (r *RabbitMQOperator) pushChannel(ctx context.Context, channelWrap *ChannelWrap) (err error) {
	if channelWrap == nil || channelWrap.channel.IsClosed() {
		return
	}
	if channelWrap.openConfirm {
		err = r.client.confirmChanPool.ReturnObject(ctx, channelWrap)
	} else {
		err = r.client.chanPool.ReturnObject(ctx, channelWrap)
	}
	return
}

func (r *RabbitMQOperator) PushDelayMessage(ctx context.Context, body *PushDelayBody, opts ...OptionFunc) (err error) {
	defer handlePanic(r)
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	if body.DelayTime == 0 && body.Expiration == "" {
		err = errors.New("no expire time set")
		return
	}
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&r.closed) == Closed {
			r.l.ErrorContext(ctx, "rabbitmq connection is closed, push cancel", "msg_id", body.MessageId)
			err = errors.New("connection is closed")
			return
		}
		pushErr := r.pushDelayMessageCore(ctx, body, opts...)
		if pushErr != nil {
			r.l.WarnContext(ctx, "[push] Push failed. after %f seconds and retry...", DefaultRetryWaitTimes.Seconds(),
				"err", pushErr.Error(), "msg_id", body.MessageId)
			select {
			case <-r.closeCh:
				r.l.ErrorContext(ctx, "[push]rmq closed, push msg cancel", "msg_id", body.MessageId)
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		r.l.InfoContext(ctx, "[push] Push msg success.", "msg_id", body.MessageId)
		return
	}
	r.l.InfoContext(ctx, fmt.Sprintf("[push] Push msg failed. msg ---> %s", string(body.Body)), "msg_id", body.MessageId)
	return
}

func (r *RabbitMQOperator) pushDelayMessageCore(ctx context.Context, body *PushDelayBody, opts ...OptionFunc) (err error) {
	delayStr := strconv.FormatInt(body.DelayTime.Milliseconds(), 10)
	delayQueue := "potato_delay_queue:" + body.ExchangeName

	channelWrap, err := r.borrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, ctx, channelWrap)

	if string(body.ExchangeType) == "" {
		body.ExchangeType = Fanout
	}
	// 注册交换机
	// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
	// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
	err = r.DeclareExchange(body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...)
	if err != nil {
		return
	}

	// 定义延迟队列(死信队列)
	_, err = r.DeclareQueue(delayQueue,
		amqp.Table{
			"x-dead-letter-exchange":    body.ExchangeName, // 指定死信交换机
			"x-dead-letter-routing-key": body.RoutingKey,   // 指定死信routing-key
		})

	// 交换机绑定队列处理
	for queue, bindingKey := range body.BindingKeyMap {
		table := body.QueueArgs[queue]
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = r.DeclareQueue(queue, table, opts...)
		if err != nil {
			return
		}

		// 队列绑定
		err = r.BindQueue(body.ExchangeName, bindingKey, queue, nil, opts...)
		if err != nil {
			return
		}
	}

	publishingMsg := amqp.Publishing{
		Timestamp:       times.Now(),
		Body:            body.Body,
		Headers:         body.Headers,
		ContentEncoding: body.ContentEncoding,
		ReplyTo:         body.ReplyTo,
		Expiration:      body.Expiration,
		MessageId:       body.MessageId,
		Type:            body.Type,
		UserId:          body.UserId,
		AppId:           body.AppId,
	}
	if body.Priority > 0 {
		publishingMsg.Priority = body.Priority
	}
	if body.DeliveryMode == 0 {
		publishingMsg.DeliveryMode = amqp.Persistent
	}
	if body.ContentType == "" {
		publishingMsg.ContentType = "text/plain"
	}
	if body.Expiration == "" {
		publishingMsg.Expiration = delayStr
	}
	// 发送消息，将消息发送到延迟队列，到期后自动路由到正常队列中
	err = channelWrap.channel.PublishWithContext(ctx, "", delayQueue, false, false, publishingMsg)
	if err != nil {
		return
	}

	if body.OpenConfirm && channelWrap.openConfirm {
		confirmed := r.confirmOne(channelWrap.confirmChan)
		if !confirmed {
			err = errors.New("push not confirmed")
			return
		}
	}
	return
}

// PushExchange 向交换机推送消息
func (r *RabbitMQOperator) PushExchange(ctx context.Context, body *ExchangePushBody, opts ...OptionFunc) (err error) {
	defer handlePanic(r)
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()
	body.Validate()

	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&r.closed) == Closed {
			r.l.ErrorContext(ctx, "rabbitmq connection is closed, push cancel", "msg_id", body.MessageId)
			err = errors.New("connection is closed")
			return
		}
		pushErr := r.pushExchangeCore(ctx, body, opts...)
		if pushErr != nil {
			r.l.WarnContext(ctx, "[push] Push failed <error:  %s>.  after %f seconds and retry... ",
				pushErr.Error(), DefaultRetryWaitTimes.Seconds(), "msg_id", body.MessageId)
			select {
			case <-r.closeCh:
				r.l.ErrorContext(ctx, "[push]rmq closed, push msg cancel", "msg_id", body.MessageId)
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		r.l.InfoContext(ctx, "[push] Push msg success.", "msg_id", body.MessageId)
		return
	}
	r.l.InfoContext(ctx, fmt.Sprintf("[push] Push msg failed. msg ---> %s", string(body.Body)), "msg_id", body.MessageId)
	return
}

// PushQueue 向队列推送消息
func (r *RabbitMQOperator) PushQueue(ctx context.Context, body *QueuePushBody, opts ...OptionFunc) (err error) {
	defer handlePanic(r)
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}

	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	body.Validate()
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&r.closed) == Closed {
			r.l.ErrorContext(ctx, "rabbitmq connection is closed, push cancel", "msg_id", body.MessageId)
			err = errors.New("connection is closed")
			return
		}
		pushErr := r.pushQueueCore(ctx, body, opts...)
		if pushErr != nil {
			r.l.WarnContext(ctx, fmt.Sprintf("[push] Push failed. after %f seconds and retry...",
				DefaultRetryWaitTimes.Seconds()), "err", pushErr.Error(), "msg_id", body.MessageId)
			select {
			case <-r.closeCh:
				r.l.ErrorContext(ctx, "[push]mq closed, push msg cancel", "msg_id", body.MessageId)
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		r.l.InfoContext(ctx, "[push] Push msg success.", "msg_id", body.MessageId)
		return
	}
	r.l.InfoContext(ctx, fmt.Sprintf("[push] Push msg failed. msg ---> %s", string(body.Body)), "msg_id", body.MessageId)
	return
}

// Push 向交换机或者队列推送消息
func (r *RabbitMQOperator) Push(ctx context.Context, body *PushBody, opts ...OptionFunc) (err error) {
	defer handlePanic(r)
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&r.closed) == Closed {
			r.l.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		pushErr := r.pushCore(ctx, body, opts...)
		if pushErr != nil {
			r.l.WarnContext(ctx, "[push] Push failed. Retrying...",
				"err", pushErr.Error(), "msg_id", body.MessageId)
			select {
			case <-r.closeCh:
				r.l.ErrorContext(ctx, "[push]mq closed, push msg cancel", "msg_id", body.MessageId)
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		r.l.InfoContext(ctx, "[push] Push msg success.", "msg_id", body.MessageId)
		return
	}
	r.l.InfoContext(ctx, fmt.Sprintf("[push] Push msg failed. msg ---> %s", string(body.Body)), "msg_id", body.MessageId)
	return
}

/*
@method: pushQueueCore
@arg: QueuePushBody ->  Args是队列的参数设置，例如优先级队列为amqp.Table{"x-max-priority":10}
@description: 向队列推送消息
*/
func (r *RabbitMQOperator) pushQueueCore(ctx context.Context, body *QueuePushBody, opts ...OptionFunc) (err error) {
	channelWrap, err := r.borrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, ctx, channelWrap)

	// 队列不存在,声明队列
	// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
	// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
	_, err = r.DeclareQueue(body.QueueName, body.Args, opts...)
	if err != nil {
		return
	}

	publishingMsg := amqp.Publishing{
		Timestamp:       times.Now(),
		Body:            body.Body,
		Headers:         body.Headers,
		ContentEncoding: body.ContentEncoding,
		ReplyTo:         body.ReplyTo,
		Expiration:      body.Expiration,
		MessageId:       body.MessageId,
		Type:            body.Type,
		UserId:          body.UserId,
		AppId:           body.AppId,
	}
	if body.Priority > 0 {
		publishingMsg.Priority = body.Priority
	}
	if body.DeliveryMode == 0 {
		publishingMsg.DeliveryMode = amqp.Persistent
	}
	if body.ContentType == "" {
		publishingMsg.ContentType = "text/plain"
	}
	// 发送任务消息
	err = channelWrap.channel.PublishWithContext(ctx, "", body.QueueName, false, false, publishingMsg)
	if err != nil {
		return
	}

	if body.OpenConfirm && channelWrap.openConfirm {
		//confirmed := r.confirmOne(channelWrap.confirmChan)
		<-channelWrap.confirmChan
		//if !confirmed {
		//	err = errors.New("push confirmed fail")
		//	return
		//}
	}
	return
}

/*
@method: pushExchangeCore
@arg: ExchangePushBody ->  Args是交换机和队列的参数设置，例如优先级队列为amqp.Table{"x-max-priority":10}
@description: 向队列推送消息
*/
func (r *RabbitMQOperator) pushExchangeCore(ctx context.Context, body *ExchangePushBody, opts ...OptionFunc) (err error) {
	channelWrap, err := r.borrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, ctx, channelWrap)

	if body.ExchangeType == "" {
		body.ExchangeType = Fanout
	}

	if len(body.QueueArgs) == 0 {
		body.QueueArgs = make(map[string]amqp.Table)
	}

	// 注册交换机
	// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
	// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
	err = r.DeclareExchange(body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...)
	if err != nil {
		return
	}

	// 交换机绑定队列处理
	for queue, bindingKey := range body.BindingKeyMap {
		table := body.QueueArgs[queue]
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = r.DeclareQueue(queue, table, opts...)
		if err != nil {
			return
		}

		// 队列绑定
		err = r.BindQueue(body.ExchangeName, bindingKey, queue, nil, opts...)
		if err != nil {
			return
		}
	}

	publishingMsg := amqp.Publishing{
		Timestamp:       times.Now(),
		Body:            body.Body,
		Headers:         body.Headers,
		ContentEncoding: body.ContentEncoding,
		ReplyTo:         body.ReplyTo,
		Expiration:      body.Expiration,
		MessageId:       body.MessageId,
		Type:            body.Type,
		UserId:          body.UserId,
		AppId:           body.AppId,
	}
	if body.Priority > 0 {
		publishingMsg.Priority = body.Priority
	}
	if body.DeliveryMode == 0 {
		publishingMsg.DeliveryMode = amqp.Persistent
	}
	if body.ContentType == "" {
		publishingMsg.ContentType = "text/plain"
	}
	// 发送任务消息
	err = channelWrap.channel.PublishWithContext(ctx, body.ExchangeName, body.RoutingKey, false, false, publishingMsg)
	if err != nil {
		return
	}

	if body.OpenConfirm && channelWrap.openConfirm {
		confirmed := r.confirmOne(channelWrap.confirmChan)
		if !confirmed {
			err = errors.New("push confirmed fail")
			return
		}
	}
	return
}

func (r *RabbitMQOperator) pushCore(ctx context.Context, body *PushBody, opts ...OptionFunc) (err error) {
	channelWrap, err := r.borrowChannel(ctx, body.OpenConfirm)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, ctx, channelWrap)

	body.Validate()

	if body.ExchangeName != "" {
		// 注册交换机
		// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
		// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
		err = r.DeclareExchange(body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...)
		if err != nil {
			r.l.ErrorContext(ctx, "MQ failed to declare the exchange", "err", err.Error(), "msg_id", body.MessageId)
			return
		}

		// 交换机绑定队列处理
		for queue, bindingKey := range body.BindingKeyMap {
			table := body.QueueArgs[queue]
			// 队列不存在,声明队列
			// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
			// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
			_, err = r.DeclareQueue(queue, table, opts...)
			if err != nil {
				r.l.ErrorContext(ctx, "MQ declare queue failed", "err", err.Error(), "msg_id", body.MessageId)
				return
			}

			// 队列绑定
			err = r.BindQueue(body.ExchangeName, bindingKey, queue, nil, opts...)
			if err != nil {
				r.l.ErrorContext(ctx, "MQ binding queue failed", "err", err.Error(), "msg_id", body.MessageId)
				return
			}
		}
	}

	if body.QueueName != "" {
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = r.DeclareQueue(body.QueueName, body.Args, opts...)
		if err != nil {
			r.l.ErrorContext(ctx, "MQ declare queue failed", "err", err.Error(), "msg_id", body.MessageId)
			return
		}
	}

	publishingMsg := amqp.Publishing{
		Timestamp:       times.Now(),
		Body:            body.Body,
		Headers:         body.Headers,
		ContentEncoding: body.ContentEncoding,
		ReplyTo:         body.ReplyTo,
		Expiration:      body.Expiration,
		MessageId:       body.MessageId,
		Type:            body.Type,
		UserId:          body.UserId,
		AppId:           body.AppId,
	}
	if body.Priority > 0 {
		publishingMsg.Priority = body.Priority
	}
	if body.DeliveryMode == 0 {
		publishingMsg.DeliveryMode = amqp.Persistent
	}
	if body.ContentType == "" {
		publishingMsg.ContentType = "text/plain"
	}
	// 发送任务消息
	err = channelWrap.channel.PublishWithContext(ctx, body.ExchangeName, body.RoutingKey, false, false, publishingMsg)
	if err != nil {
		r.l.ErrorContext(ctx, "MQ task failed to be sent", "err", err.Error(), "msg_id", body.MessageId)
		return
	}
	if body.OpenConfirm && channelWrap.openConfirm {
		confirmed := r.confirmOne(channelWrap.confirmChan)
		if !confirmed {
			err = errors.New("push confirmed fail")
			return
		}
	}
	return
}

func (r *RabbitMQOperator) Consume(ctx context.Context, param *ConsumeBody) (<-chan amqp.Delivery, error) {
	var channelWrap *ChannelWrap
	resChan, consumerTag, err := r.consumeCore(ctx, param, &channelWrap)
	if err != nil {
		return nil, err
	}

	contents := make(chan amqp.Delivery, 5)

	go func() {
		ctxBack := context.Background()
		defer func() {
			if e := recover(); e != nil {
				r.l.ErrorContext(ctxBack, fmt.Sprintf("recover_panic: %v", e))
			}
		}()

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

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

		ticker := time.NewTicker(DefaultRetryWaitTimes)
		defer ticker.Stop()
		var innerErr error
		var innerConsume bool

	process:
		if innerConsume {
			resChan, consumerTag, innerErr = r.consumeCore(ctxBack, param, &channelWrap)
			// Keep retrying when consumption fails
			for innerErr != nil {
				r.l.ErrorContext(ctxBack, fmt.Sprintf("[consume:%s] consume msg error, retrying ...", param.QueueName), "err", innerErr.Error())
				select {
				case s := <-sig:
					r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] recived： %v, exiting... ", param.QueueName, s))
					return
				case <-r.closeCh:
					r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] rabbitmq is closed, exiting...", param.QueueName))
					return
				case <-valueCh.(chan bool):
					r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] consume cancel, exiting...", param.QueueName))
					_ = channelWrap.Close()
					return
				case <-ticker.C:
				}
				resChan, consumerTag, innerErr = r.consumeCore(ctxBack, param, &channelWrap)
			}
		} else {
			innerConsume = true
		}

		// Circular consumption of data
		for {
			select {
			case s := <-sig:
				r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] recived： %v, exiting... ", param.QueueName, s))
				return
			case <-r.closeCh:
				r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] rabbitmq is closed, exiting...", param.QueueName))
				return
			case <-valueCh.(chan bool):
				r.l.Info("[consume:%s] consume cancel, exiting...", param.QueueName)
				cancelErr := channelWrap.channel.Cancel(consumerTag, false)
				if cancelErr != nil {
					r.l.ErrorContext(ctx, fmt.Sprintf("[consume:%s] consume cancel error, exiting...", param.QueueName), "err", cancelErr.Error())
				}
				r.client.cancelChan.Delete(param.QueueName)
				_ = channelWrap.Close()
				return
			case <-channelWrap.closeChan:
				r.l.WarnContext(ctx, fmt.Sprintf("[consume:%s] rmq channel is closed, reconsume...", param.QueueName))
				goto process
			case item := <-resChan:
				if len(item.Body) == 0 {
					continue
				}
				select {
				case s := <-sig:
					r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] recived： %v, exiting... ", param.QueueName, s))
					return
				case <-r.closeCh:
					r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] rabbitmq is closed, exiting...", param.QueueName))
					return
				case <-channelWrap.closeChan:
					r.l.WarnContext(ctx, fmt.Sprintf("[consume:%s] rmq channel is closed, reconsume...", param.QueueName))
					goto process
				case contents <- item:
					r.l.InfoContext(ctx, fmt.Sprintf("[consume:%s] recived msg success: msg_id --> %s", param.QueueName, item.MessageId))
				}
			case <-ticker.C:
				continue
			}
		}

	}()
	return contents, nil
}

func (r *RabbitMQOperator) consumeCore(ctx context.Context, param *ConsumeBody, channelWrap **ChannelWrap,
	opts ...OptionFunc) (contents <-chan amqp.Delivery, consumerTag string, err error) {
	if !r.isReady {
		err = errors.New("connection is not ready")
		return
	}
	if *channelWrap != nil {
		_ = (*channelWrap).Close()
	}
	*channelWrap, err = r.borrowChannel(ctx, false)
	if err != nil {
		return
	}
	(*channelWrap).isConsumer = true
	err = r.recheck(*channelWrap)
	if err != nil {
		return
	}

	if param.FetchCount == 0 {
		param.FetchCount = 1
	}

	// 队列不存在,声明队列
	// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
	// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
	_, err = r.DeclareQueue(param.QueueName, param.QueueArgs, opts...)
	if err != nil {
		return
	}
	if err != nil {
		r.l.ErrorContext(ctx, "MQ declare queue failed", "err", err.Error())
		return
	}
	if param.ExchangeName != "" {
		// 绑定任务
		err = r.BindQueue(param.ExchangeName, param.RoutingKey, param.QueueName, nil, opts...)
		if err != nil {
			r.l.ErrorContext(ctx, "binding queue failed", "err", err.Error())
			return
		}
	}

	// 获取消费通道,确保rabbitMQ一个一个发送消息
	err = (*channelWrap).channel.Qos(param.FetchCount, 0, true)
	if err != nil {
		r.l.ErrorContext(ctx, "open qos error", "err", err.Error())
		return
	}
	var args amqp.Table
	if param.XPriority != 0 {
		args = amqp.Table{"x-priority": param.XPriority}
	}
	consumerTag = uniqueConsumerTag()
	contents, err = (*channelWrap).channel.Consume(param.QueueName, consumerTag, param.AutoAck, false, false, false, args)
	if err != nil {
		r.l.ErrorContext(ctx, "The acquisition of the consumption channel is abnormal", "err", err.Error())
		return
	}
	return
}

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

func (r *RabbitMQOperator) DeclareExchange(exchangeName, exchangeType string, args amqp.Table, opts ...OptionFunc) (err error) {
	options := &Options{
		durable:    true,
		autoDelete: false,
		internal:   false,
		noWait:     false,
	}
	for _, opt := range opts {
		opt(options)
	}
	channelWrap, err := r.borrowChannel(context.Background(), false)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, context.Background(), channelWrap)

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

func (r *RabbitMQOperator) DeclareQueue(queueName string, args amqp.Table, opts ...OptionFunc) (queue amqp.Queue, err error) {
	options := &Options{
		durable:    true,
		autoDelete: false,
		exclusive:  false,
		noWait:     false,
	}
	for _, opt := range opts {
		opt(options)
	}
	channelWrap, err := r.borrowChannel(context.Background(), false)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, context.Background(), channelWrap)

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
	zapx.GetLogger().Warn("cancel consumer timeout：%s", queueName)
	return
}

func (r *RabbitMQOperator) BindQueue(exchangeName, routingKey, queueName string, args amqp.Table, opts ...OptionFunc) (err error) {
	options := &Options{
		noWait: false,
	}
	channelWrap, err := r.borrowChannel(context.Background(), false)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, context.Background(), channelWrap)
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

func (r *RabbitMQOperator) UnBindQueue(exchangeName, queueName, routingKey string, args amqp.Table) (err error) {
	channelWrap, err := r.borrowChannel(context.Background(), false)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, context.Background(), channelWrap)

	err = channelWrap.channel.QueueUnbind(queueName, routingKey, exchangeName, args)
	if err != nil {
		return err
	}

	opKey := fmt.Sprintf("bind_queue:%s_%s", exchangeName, queueName)
	r.opCache.Delete(opKey)
	return
}

func (r *RabbitMQOperator) DeleteExchange(exchangeName string, opts ...OptionFunc) (err error) {
	options := &Options{
		ifUnUsed: true,
		noWait:   false,
	}
	for _, opt := range opts {
		opt(options)
	}
	channelWrap, err := r.borrowChannel(context.Background(), false)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, context.Background(), channelWrap)
	err = channelWrap.channel.ExchangeDelete(exchangeName, options.ifUnUsed, options.noWait)
	if err != nil {
		return err
	}
	r.opCache.Delete(fmt.Sprintf("declare_exchange:%s", exchangeName))
	return
}

func (r *RabbitMQOperator) DeleteQueue(queueName string, opts ...OptionFunc) (err error) {
	options := &Options{
		ifUnUsed: true,
		ifEmpty:  true,
		noWait:   false,
	}
	for _, opt := range opts {
		opt(options)
	}

	channelWrap, err := r.borrowChannel(context.Background(), false)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, context.Background(), channelWrap)
	_, err = channelWrap.channel.QueueDelete(queueName, options.ifUnUsed, options.ifEmpty, options.noWait)
	if err != nil {
		return err
	}
	r.opCache.Delete(fmt.Sprintf("declare_queue:%s", queueName))
	return
}

func (r *RabbitMQOperator) GetMessageCount(queueName string) (count int, err error) {
	channelWrap, err := r.borrowChannel(context.Background(), false)
	if err != nil {
		return
	}

	defer func(r *RabbitMQOperator, ctx context.Context, channelWrap *ChannelWrap) {
		_ = r.pushChannel(ctx, channelWrap)
	}(r, context.Background(), channelWrap)

	queue, err := channelWrap.channel.QueueInspect(queueName)
	if err != nil {
		return
	}
	count = queue.Messages
	return
}

func (r *RabbitMQOperator) Close() (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if atomic.LoadInt32(&r.closed) == Closed {
		return
	}

	if !r.client.conn.IsClosed() {
		err = r.client.conn.Close()
		if err != nil {
			return
		}
	}

	atomic.StoreInt32(&r.closed, Closed)
	close(r.closeCh)
	r.isReady = false
	return
}

func (r *RabbitMQOperator) Ack(ctx context.Context, msg amqp.Delivery) {
	ch := make(chan bool, 1)
	defer close(ch)
	go func() {
		err := msg.Ack(false)
		if err != nil {
			ch <- false
			return
		}
		ch <- true
	}()
	select {
	case <-r.closeCh:
		r.l.InfoContext(ctx, "[Consume]mq closed, ack cancel for: "+msg.RoutingKey, "msg_id", msg.MessageId)
		return
	case done := <-ch:
		if done {
			r.l.InfoContext(ctx, "[Ack]ack done", "msg_id", msg.MessageId)
		} else {
			r.l.InfoContext(ctx, "[Ack]ack failed", "msg_id", msg.MessageId)
		}
	case <-time.After(DefaultRetryWaitTimes * time.Second):
		r.l.InfoContext(ctx, "[Ack]ack timeout", "msg_id", msg.MessageId)
	}
}

func (r *RabbitMQOperator) Nack(ctx context.Context, msg amqp.Delivery) {
	ch := make(chan bool, 1)
	defer close(ch)
	go func() {
		err := msg.Nack(false, true)
		if err != nil {
			ch <- false
			return
		}
		ch <- true
	}()
	select {
	case <-r.closeCh:
		r.l.InfoContext(ctx, "[Consume]mq closed, nack cancel for: "+msg.RoutingKey, "msg_id", msg.MessageId)
		return
	case done := <-ch:
		if done {
			r.l.InfoContext(ctx, "[Nack]Nack done", "msg_id", msg.MessageId)
		} else {
			r.l.InfoContext(ctx, "[Nack]Nack failed", "msg_id", msg.MessageId)
		}
		close(ch)
	case <-time.After(DefaultRetryWaitTimes * time.Second):
		r.l.InfoContext(ctx, "[Nack]Nack timeout", "msg_id", msg.MessageId)
	}
}
