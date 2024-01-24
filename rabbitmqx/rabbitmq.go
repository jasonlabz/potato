package rabbitmqx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jasonlabz/potato/core/config/application"
	"github.com/jasonlabz/potato/core/times"
	"github.com/jasonlabz/potato/core/utils"
	log "github.com/jasonlabz/potato/log/zapx"
)

var operator *RabbitMQOperator

func GetRabbitMQOperator() *RabbitMQOperator {
	return operator
}

func init() {
	appConf := application.GetConfig()
	if appConf.Rabbitmq != nil && len(appConf.Rabbitmq.Host) > 0 {
		mqConf := &MQConfig{}
		err := utils.CopyStruct(appConf.Rabbitmq, mqConf)
		if err != nil {
			log.DefaultLogger().WithError(err).Error("copy rmq config error, skipping ...")
			return
		}
		err = InitRabbitMQOperator(mqConf)
		if err != nil {
			log.DefaultLogger().WithError(err).Error("init rmq Client error, skipping ...")
		}
	}
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
	DefaultRetryWaitTimes    = 2 * time.Second
	DefaultConsumeRetryTimes = 3 * time.Second
	Closed                   = 1
	RetryTimes               = 3
)

type ExchangeType string

const (
	ExchangeTypeDirect ExchangeType = "direct" // 直连
	ExchangeTypeFanout ExchangeType = "fanout" // 广播
	ExchangeTypeTopic  ExchangeType = "topic"  // 通配符
)

// NewRabbitMQOperator 该函数负责根据配置创建rmq客户端对象供外部调用
func NewRabbitMQOperator(config *MQConfig) (op *RabbitMQOperator, err error) {
	logger := log.DefaultLogger()
	op = &RabbitMQOperator{}
	// init
	op.client = &Client{
		closeConnNotify: make(chan *amqp.Error, 1),
	}
	op.closeCh = make(chan bool)
	op.name = fmt.Sprintf("rmq_%v", rand.Int31())

	// validate config param
	if err = config.Validate(); err != nil {
		return
	}
	op.config = config

	// ready connect rmq
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()
	for i := 0; i < RetryTimes; i++ {
		op.client.conn, err = amqp.DialTLS(config.Addr(), &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			<-ticker.C
			logger.Warn("wait %f seconds for retry to connect...", DefaultRetryWaitTimes.Seconds())
		}
	}
	if err != nil {
		return
	}

	op.client.commonCh, err = op.client.conn.Channel()
	if err != nil {
		op.Close()
		return
	}
	op.isReady = true
	logger.Info("rabbitmq init success[addr:%s]", config.Addr())
	// a new goroutine for check disconnect
	go op.tryReConnect(true)

	return
}

// 消息确认
func (op *RabbitMQOperator) confirmOne(confirms <-chan amqp.Confirmation) (ok bool) {
	logger := log.DefaultLogger()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

	select {
	case s := <-sig:
		logger.Info("recived： %v, exiting... ", s)
		return
	case <-op.closeCh:
		logger.Info("rabbitmq is closed, exiting...")
		return
	case confirmed := <-confirms:
		if confirmed.Ack {
			ok = true
		} else {
			logger.Warn("confirmed delivery false of delivery tag: %d", confirmed.DeliveryTag)
		}
	}
	return
}

func handlePanic() {
	if r := recover(); r != nil {
		log.DefaultLogger().Error("Recovered: %+v", r)
	}
}

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

func (c *MQConfig) Validate() error {
	if c.Username == "" {
		return errors.New("username is empty")
	}
	if c.Password == "" {
		return errors.New("password is empty")
	}
	if c.Host == "" {
		return errors.New("host is empty")
	}
	if c.Port == 0 {
		return errors.New("port is empty")
	}
	return nil
}

func (c *MQConfig) Addr() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", c.Username, c.Password, c.Host, c.Port)
}

type RabbitMQOperator struct {
	name    string
	config  *MQConfig
	opCache sync.Map
	client  *Client
	closeCh chan bool
	isReady bool
	closed  int32
	mu      sync.Mutex
}

type Client struct {
	conn                *amqp.Connection
	commonCh            *amqp.Channel
	channelCache        sync.Map
	pushConfirmListener sync.Map
	chCloseListener     sync.Map
	cacheMu             sync.Mutex
	closeConnNotify     chan *amqp.Error
}

func (op *RabbitMQOperator) SetLogger(logger amqp.Logging) {
	amqp.Logger = logger
}

func (op *RabbitMQOperator) tryReConnect(daemon bool) (connected bool) {
	logger := log.DefaultLogger()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	for {
		if !op.isReady && atomic.LoadInt32(&op.closed) != Closed {
			conn, err := amqp.DialTLS(op.config.Addr(), &tls.Config{InsecureSkipVerify: true})
			if err == nil {
				op.client.conn = conn
				op.client.conn.NotifyClose(op.client.closeConnNotify)
				op.isReady = true
				logger.Info("rabbitmq reconnect success[addr:%s]", op.config.Addr())
				continue
			}
			select {
			case s := <-sig:
				logger.Info("recived： %v, exiting... ", s)
				return
			case <-op.closeCh:
				logger.Info("rabbitmq is closed, exiting...")
				return
			case <-ticker.C:
			}
		}

		if !daemon {
			connected = true
			break
		}

		select {
		case s := <-sig:
			logger.Info("recived： %v, exiting daemon program... ", s)
			return
		case <-op.closeCh:
			logger.Info("rabbitmq is closed, exiting daemon program...")
			return
		case <-op.client.closeConnNotify:
			op.isReady = false
			op.client.channelCache = sync.Map{}
			op.client.chCloseListener = sync.Map{}
			logger.Error("rabbitmq is disconnect, retrying...")
		}
	}
	return
}

func (op *RabbitMQOperator) getChannelForExchange(exchange string, order bool) (key string, channel *amqp.Channel, err error) {
	key = exchange + "_exchange:publish"

	ch, ok := op.client.channelCache.Load(key)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()

	ch, ok = op.client.channelCache.Load(key)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}
	if op.client.conn.IsClosed() {
		time.Sleep(1 * time.Second)
		if op.client.conn.IsClosed() {
			err = errors.New("rabbitmq is disconnected")
			return
		}
	}
	channel, err = op.client.conn.Channel()
	if err != nil {
		return
	}
	op.client.channelCache.Store(key, channel)

	if order {
		pushConfirm := make(chan amqp.Confirmation, 1)
		channel.NotifyPublish(pushConfirm)
		op.client.pushConfirmListener.Store(key, pushConfirm)
	}

	return
}

func (op *RabbitMQOperator) getChannelForQueue(isConsume bool, queue string, order bool) (key string, channel *amqp.Channel, err error) {
	if isConsume {
		key = queue + "_queue:consume"
	} else {
		key = queue + "_queue:publish"
	}
	ch, ok := op.client.channelCache.Load(key)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()

	ch, ok = op.client.channelCache.Load(key)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}
	if op.client.conn.IsClosed() {
		time.Sleep(1 * time.Second)
		if op.client.conn.IsClosed() {
			err = errors.New("rabbitmq is disconnected")
			return
		}
	}
	channel, err = op.client.conn.Channel()
	if err != nil {
		return
	}
	op.client.channelCache.Store(key, channel)

	if !isConsume && order {
		pushConfirm := make(chan amqp.Confirmation, 1)
		channel.NotifyPublish(pushConfirm)
		op.client.pushConfirmListener.Store(key, pushConfirm)
	}

	if isConsume {
		closeErr := make(chan *amqp.Error, 1)
		channel.NotifyClose(closeErr)
		op.client.chCloseListener.Store(key, closeErr)
	}
	return
}

func (op *RabbitMQOperator) getChannel(isConsume bool, exchange, queue string, order bool) (key string, channel *amqp.Channel, err error) {
	if isConsume {
		key, channel, err = op.getChannelForQueue(isConsume, queue, order)
		return
	}
	if exchange != "" {
		key, channel, err = op.getChannelForExchange(exchange, order)
		return
	}
	key, channel, err = op.getChannelForQueue(false, queue, order)
	return
}

func (op *RabbitMQOperator) PushDelayMessage(ctx context.Context, body *PushDelayBody, opts ...OptionFunc) (err error) {
	defer handlePanic()
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	if body.DelayTime == 0 && body.Expiration == "" {
		err = errors.New("no expire time set")
		return
	}
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		pushErr := op.pushDelayMessageCore(ctx, body, opts...)
		if pushErr != nil {
			logger.WithError(pushErr).Warn("[push] Push failed. after %f seconds and retry...", DefaultRetryWaitTimes.Seconds())
			select {
			case <-op.closeCh:
				logger.Error("[push]rmq closed, push msg cancel")
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg --->  %s", string(body.Body))
	return
}

func (op *RabbitMQOperator) pushDelayMessageCore(ctx context.Context, body *PushDelayBody, opts ...OptionFunc) (err error) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))

	delayStr := strconv.FormatInt(body.DelayTime.Milliseconds(), 10)
	delayQueue := "potato_delay_queue:" + body.ExchangeName
	//delayRouteKey := body.RoutingKey + "_delay:" + delayStr

	key, channel, err := op.getChannelForExchange(body.ExchangeName, body.ConfirmedByOrder)
	if err != nil {
		return
	}

	if body.OpenConfirm {
		err = channel.Confirm(false)
		if err != nil {
			return
		}
	}
	if string(body.ExchangeType) == "" {
		body.ExchangeType = ExchangeTypeFanout
	}
	// 注册交换机
	// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
	// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
	err = op.DeclareExchange(body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...)
	if err != nil {
		return
	}

	// 定义延迟队列(死信队列)
	_, err = op.DeclareQueue(delayQueue,
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
		_, err = op.DeclareQueue(queue, table, opts...)
		if err != nil {
			return
		}

		// 队列绑定
		err = op.BindQueue(body.ExchangeName, bindingKey, queue, nil, opts...)
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
	err = channel.PublishWithContext(ctx, "", delayQueue, false, false, publishingMsg)
	if err != nil {
		return
	}

	if body.ConfirmedByOrder {
		confirmCh, ok := op.client.pushConfirmListener.Load(key)
		if !ok {
			logger.Warn("msg pushed, but has no confirm channel, skip confirm ...")
			return
		}

		confirmed := op.confirmOne(confirmCh.(chan amqp.Confirmation))
		if !confirmed {
			err = errors.New("push confirmed fail")
			return
		}
	}
	return
}

// PushExchange 向交换机推送消息
func (op *RabbitMQOperator) PushExchange(ctx context.Context, body *ExchangePushBody, opts ...OptionFunc) (err error) {
	defer handlePanic()
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()
	body.Validate()

	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		pushErr := op.pushExchangeCore(ctx, body, opts...)
		if pushErr != nil {
			logger.Warn("[push] Push failed <error:  %s>.  after %f seconds and retry... ", pushErr.Error(), DefaultRetryWaitTimes.Seconds())
			select {
			case <-op.closeCh:
				logger.Error("[push]rmq closed, push msg cancel")
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg ---> %s", string(body.Body))
	return
}

// PushQueue 向队列推送消息
func (op *RabbitMQOperator) PushQueue(ctx context.Context, body *QueuePushBody, opts ...OptionFunc) (err error) {
	defer handlePanic()
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()

	body.Validate()
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		pushErr := op.pushQueueCore(ctx, body, opts...)
		if pushErr != nil {
			logger.WithError(pushErr).Warn("[push] Push failed. after %f seconds and retry...", DefaultRetryWaitTimes.Seconds())
			select {
			case <-op.closeCh:
				logger.Error("[push]mq closed, push msg cancel")
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg ---> %s", string(body.Body))
	return
}

// Push 向交换机或者队列推送消息
func (op *RabbitMQOperator) Push(ctx context.Context, body *PushBody, opts ...OptionFunc) (err error) {
	defer handlePanic()
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	ticker := time.NewTicker(DefaultRetryWaitTimes)
	defer ticker.Stop()
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		pushErr := op.pushCore(ctx, body, opts...)
		if pushErr != nil {
			logger.WithError(pushErr).Warn("[push] Push failed. Retrying...")
			select {
			case <-op.closeCh:
				logger.Error("[push]mq closed, push msg cancel")
				err = errors.New("rabbitmq connection closed")
				return
			case <-ticker.C:
			}
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg ---> %s", string(body.Body))
	return
}

/*
@method: pushQueueCore
@arg: QueuePushBody ->  Args是队列的参数设置，例如优先级队列为amqp.Table{"x-max-priority":10}
@description: 向队列推送消息
*/
func (op *RabbitMQOperator) pushQueueCore(ctx context.Context, body *QueuePushBody, opts ...OptionFunc) (err error) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))

	key, channel, err := op.getChannelForQueue(false, body.QueueName, body.ConfirmedByOrder)
	if err != nil {
		return
	}

	if body.OpenConfirm {
		err = channel.Confirm(false)
		if err != nil {
			return
		}
	}

	// 队列不存在,声明队列
	// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
	// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
	_, err = op.DeclareQueue(body.QueueName, body.Args, opts...)
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
	err = channel.PublishWithContext(ctx, "", body.QueueName, false, false, publishingMsg)
	if err != nil {
		return
	}

	if body.ConfirmedByOrder {
		confirmCh, ok := op.client.pushConfirmListener.Load(key)
		if !ok {
			logger.Warn("msg pushed, but has no confirm channel, skip confirm ...")
			return
		}

		confirmed := op.confirmOne(confirmCh.(chan amqp.Confirmation))
		if !confirmed {
			err = errors.New("push confirmed fail")
			return
		}
	}
	return
}

/*
@method: pushExchangeCore
@arg: ExchangePushBody ->  Args是交换机和队列的参数设置，例如优先级队列为amqp.Table{"x-max-priority":10}
@description: 向队列推送消息
*/
func (op *RabbitMQOperator) pushExchangeCore(ctx context.Context, body *ExchangePushBody, opts ...OptionFunc) (err error) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))

	key, channel, err := op.getChannelForExchange(body.ExchangeName, body.ConfirmedByOrder)
	if err != nil {
		return
	}

	if body.OpenConfirm {
		err = channel.Confirm(false)
		if err != nil {
			return
		}
	}

	if body.ExchangeType == "" {
		body.ExchangeType = ExchangeTypeFanout
	}

	if len(body.QueueArgs) == 0 {
		body.QueueArgs = make(map[string]amqp.Table)
	}

	// 注册交换机
	// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
	// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
	err = op.DeclareExchange(body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...)
	if err != nil {
		return
	}

	// 交换机绑定队列处理
	for queue, bindingKey := range body.BindingKeyMap {
		table := body.QueueArgs[queue]
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = op.DeclareQueue(queue, table, opts...)
		if err != nil {
			return
		}

		// 队列绑定
		err = op.BindQueue(body.ExchangeName, bindingKey, queue, nil, opts...)
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
	err = channel.PublishWithContext(ctx, body.ExchangeName, body.RoutingKey, false, false, publishingMsg)
	if err != nil {
		return
	}

	if body.ConfirmedByOrder {
		confirmCh, ok := op.client.pushConfirmListener.Load(key)
		if !ok {
			logger.Warn("msg pushed, but has no confirm channel, skip confirm ...")
			return
		}

		confirmed := op.confirmOne(confirmCh.(chan amqp.Confirmation))
		if !confirmed {
			err = errors.New("push confirmed fail")
			return
		}
	}
	return
}

func (op *RabbitMQOperator) pushCore(ctx context.Context, body *PushBody, opts ...OptionFunc) (err error) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	key, channel, err := op.getChannel(false, body.ExchangeName, body.QueueName, body.ConfirmedByOrder)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if body.OpenConfirm {
		err = channel.Confirm(false)
		if err != nil {
			logger.Error(err.Error())
			return
		}
	}

	body.Validate()

	if body.ExchangeName != "" {
		// 注册交换机
		// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
		// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
		err = op.DeclareExchange(body.ExchangeName, string(body.ExchangeType), body.ExchangeArgs, opts...)
		if err != nil {
			logger.WithError(err).Error("MQ failed to declare the exchange")
			return
		}

		// 交换机绑定队列处理
		for queue, bindingKey := range body.BindingKeyMap {
			table := body.QueueArgs[queue]
			// 队列不存在,声明队列
			// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
			// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
			_, err = op.DeclareQueue(queue, table, opts...)
			if err != nil {
				logger.WithError(err).Error("MQ declare queue failed")
				return
			}

			// 队列绑定
			err = op.BindQueue(body.ExchangeName, bindingKey, queue, nil, opts...)
			if err != nil {
				logger.WithError(err).Error("MQ binding queue failed")
				return
			}
		}
	}

	if body.QueueName != "" {
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = op.DeclareQueue(body.QueueName, body.Args, opts...)
		if err != nil {
			logger.WithError(err).Error("MQ declare queue failed")
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
	err = channel.PublishWithContext(ctx, body.ExchangeName, body.RoutingKey, false, false, publishingMsg)
	if err != nil {
		logger.WithError(err).Error("MQ task failed to be sent")
		return
	}
	if body.ConfirmedByOrder {
		confirmCh, ok := op.client.pushConfirmListener.Load(key)
		if !ok {
			logger.Warn("msg pushed, but has no confirm channel, skip confirm ...")
			return
		}

		confirmed := op.confirmOne(confirmCh.(chan amqp.Confirmation))
		if !confirmed {
			err = errors.New("push confirmed fail")
			return
		}
	}
	return
}

func (op *RabbitMQOperator) Consume(ctx context.Context, param *ConsumeBody) (<-chan amqp.Delivery, error) {
	logger := log.GetLogger(ctx)
	resChan, key, err := op.consumeCore(ctx, param)
	if err != nil {
		return nil, err
	}
	contents := make(chan amqp.Delivery, 3)

	go func() {
		ctxBack := context.Background()
		rLogger := log.GetLogger(ctxBack)
		defer func() {
			if e := recover(); e != nil {
				logger.Error("recover_panic: %v", e)
			}
		}()

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

		ticker := time.NewTicker(DefaultRetryWaitTimes)
		defer ticker.Stop()
		var innerErr error
		var innerConsume bool

	process:
		if innerConsume {
			resChan, key, innerErr = op.consumeCore(ctxBack, param)
			// Keep retrying when consumption fails
			for innerErr != nil {
				rLogger.WithError(innerErr).Error("[consume:%s] consume msg error, retrying ...", param.QueueName)
				select {
				case s := <-sig:
					rLogger.Info("[consume:%s] recived： %v, exiting... ", param.QueueName, s)
					return
				case <-op.closeCh:
					rLogger.Info("[consume:%s] rabbitmq is closed, exiting...", param.QueueName)
					return
				case <-ticker.C:
				}
				resChan, key, innerErr = op.consumeCore(ctxBack, param)
			}
		} else {
			innerConsume = true
		}

		// Circular consumption of data
		listenCh, ok := op.client.chCloseListener.Load(key)
		if !ok {
			rLogger.Warn("[consume:%s] has no listen channel, retring...", param.QueueName)
			goto process
		}
		for {
			select {
			case s := <-sig:
				rLogger.Info("[consume:%s] recived： %v, exiting... ", param.QueueName, s)
				return
			case <-op.closeCh:
				rLogger.Info("[consume:%s] rabbitmq is closed, exiting...", param.QueueName)
				return
			case <-listenCh.(chan *amqp.Error):
				rLogger.Warn("[consume:%s] rmq channel is closed, reconsume...", param.QueueName)
				goto process
			case item := <-resChan:
				if len(item.Body) == 0 {
					continue
				}
				select {
				case s := <-sig:
					rLogger.Info("[consume:%s] recived： %v, exiting... ", param.QueueName, s)
					return
				case <-op.closeCh:
					rLogger.Info("[consume:%s] rabbitmq is closed, exiting...", param.QueueName)
					return
				case <-listenCh.(chan *amqp.Error):
					rLogger.Warn("[consume:%s] rmq channel is closed, reconsume...", param.QueueName)
					goto process
				case contents <- item:
					rLogger.Info("[consume:%s] recived msg success: msg_id --> %s", param.QueueName, item.MessageId)
				}
			case <-ticker.C:
				continue
			}
		}

	}()
	return contents, nil
}

func (op *RabbitMQOperator) consumeCore(ctx context.Context, param *ConsumeBody, opts ...OptionFunc) (contents <-chan amqp.Delivery, key string, err error) {
	logger := log.GetLogger(ctx)
	if !op.isReady {
		err = errors.New("connection is not ready")
		return
	}
	var channel *amqp.Channel
	key, channel, err = op.getChannel(true, param.ExchangeName, param.QueueName, false)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	if param.FetchCount == 0 {
		param.FetchCount = 1
	}

	// 队列不存在,声明队列
	// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
	// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
	_, err = op.DeclareQueue(param.QueueName, param.QueueArgs, opts...)
	if err != nil {
		return
	}
	if err != nil {
		logger.WithError(err).Error("MQ declare queue failed")
		return
	}
	if param.ExchangeName != "" {
		// 绑定任务
		err = op.BindQueue(param.ExchangeName, param.RoutingKey, param.QueueName, nil, opts...)
		if err != nil {
			logger.WithError(err).Error("binding queue failed")
			return
		}
	}

	// 获取消费通道,确保rabbitMQ一个一个发送消息
	err = channel.Qos(param.FetchCount, 0, true)
	if err != nil {
		logger.WithError(err).Error("open qos error")
		return
	}
	var args amqp.Table
	if param.XPriority != 0 {
		args = amqp.Table{"x-priority": param.XPriority}
	}
	contents, err = channel.Consume(param.QueueName, "", param.AutoAck, false, false, false, args)
	if err != nil {
		logger.WithError(err).Error("The acquisition of the consumption channel is abnormal")
		return
	}
	return
}

func (op *RabbitMQOperator) releaseExchangeChannel(exchangeName string) (err error) {
	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()
	exchangeName += "_exchange:publish"
	value, ok := op.client.channelCache.Load(exchangeName)
	if ok && !value.(*amqp.Channel).IsClosed() {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
	}
	op.client.channelCache.Delete(exchangeName)
	op.client.pushConfirmListener.Delete(exchangeName)

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
func WithDelOptionIfEmpty(ifUnUsed bool) OptionFunc {
	return func(options *Options) {
		options.ifUnUsed = ifUnUsed
	}
}
func WithDelOptionIfUnused(ifUnUsed bool) OptionFunc {
	return func(options *Options) {
		options.ifUnUsed = ifUnUsed
	}
}

func (op *RabbitMQOperator) DeclareExchange(exchangeName, exchangeType string, args amqp.Table, opts ...OptionFunc) (err error) {
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
	_, ok := op.opCache.Load(opKey)
	if ok {
		options.noWait = true
	}

	err = op.checkCommonChannel()
	if err != nil {
		return
	}
	err = op.client.commonCh.ExchangeDeclarePassive(exchangeName, exchangeType, options.durable, options.autoDelete, options.internal, options.noWait, args)
	if err == nil {
		if !ok {
			op.opCache.Store(opKey, true)
		}
		return
	}

	err = op.checkCommonChannel()
	if err != nil {
		return
	}
	err = op.client.commonCh.ExchangeDeclare(exchangeName, exchangeType, options.durable, options.autoDelete, options.internal, options.noWait, args)
	if err != nil {
		return
	}

	if !ok {
		op.opCache.Store(opKey, true)
	}
	return
}

func (op *RabbitMQOperator) DeclareQueue(queueName string, args amqp.Table, opts ...OptionFunc) (queue amqp.Queue, err error) {
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
	_, ok := op.opCache.Load(opKey)
	if ok {
		options.noWait = true
	}

	err = op.checkCommonChannel()
	if err != nil {
		return
	}
	queue, err = op.client.commonCh.QueueDeclarePassive(queueName, options.durable, options.autoDelete, options.exclusive, options.noWait, args)
	if err == nil {
		if !ok {
			op.opCache.Store(opKey, true)
		}
		return
	}

	err = op.checkCommonChannel()
	if err != nil {
		return
	}
	queue, err = op.client.commonCh.QueueDeclare(queueName, options.durable, options.autoDelete, options.exclusive, options.noWait, args)
	if err != nil {
		return
	}

	if !ok {
		op.opCache.Store(opKey, true)
	}
	return
}

func (op *RabbitMQOperator) BindQueue(exchangeName, routingKey, queueName string, args amqp.Table, opts ...OptionFunc) (err error) {
	err = op.checkCommonChannel()
	if err != nil {
		return
	}
	options := &Options{
		noWait: false,
	}

	opKey := fmt.Sprintf("bind_queue:%s_%s", exchangeName, queueName)
	_, ok := op.opCache.Load(opKey)
	if ok {
		options.noWait = true
	}

	for _, opt := range opts {
		opt(options)
	}
	err = op.client.commonCh.QueueBind(queueName, routingKey, exchangeName, options.noWait, args)
	if err != nil {
		return err
	}

	if !ok {
		op.opCache.Store(opKey, true)
	}
	return
}

func (op *RabbitMQOperator) UnBindQueue(exchangeName, queueName, routingKey string, args amqp.Table) (err error) {
	err = op.checkCommonChannel()
	if err != nil {
		return
	}

	err = op.client.commonCh.QueueUnbind(queueName, routingKey, exchangeName, args)
	if err != nil {
		return err
	}

	opKey := fmt.Sprintf("bind_queue:%s_%s", exchangeName, queueName)
	op.opCache.Delete(opKey)
	return
}

func (op *RabbitMQOperator) DeleteExchange(exchangeName string, opts ...OptionFunc) (err error) {
	options := &Options{
		ifUnUsed: true,
		noWait:   true,
	}
	for _, opt := range opts {
		opt(options)
	}

	err = op.checkCommonChannel()
	if err != nil {
		return
	}
	err = op.client.commonCh.ExchangeDelete(exchangeName, options.ifUnUsed, options.noWait)
	if err != nil {
		return err
	}
	go func() {
		opKey := fmt.Sprintf("declare_exchange:%s", exchangeName)
		op.opCache.Delete(opKey)
		_ = op.releaseExchangeChannel(exchangeName)
	}()
	return
}

func (op *RabbitMQOperator) DeleteQueue(queueName string, opts ...OptionFunc) (err error) {
	options := &Options{
		ifUnUsed: true,
		ifEmpty:  true,
		noWait:   true,
	}
	for _, opt := range opts {
		opt(options)
	}

	err = op.checkCommonChannel()
	if err != nil {
		return
	}
	_, err = op.client.commonCh.QueueDelete(queueName, options.ifUnUsed, options.ifEmpty, options.noWait)
	if err != nil {
		return err
	}
	go func() {
		opKey := fmt.Sprintf("declare_queue:%s", queueName)
		op.opCache.Delete(opKey)
		_ = op.releaseQueueChannel(queueName)
	}()
	return
}

func (op *RabbitMQOperator) releaseQueueChannel(queueName string) (err error) {
	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()

	publishQueue := queueName + "_queue:publish"
	value1, ok1 := op.client.channelCache.Load(publishQueue)
	if ok1 && !value1.(*amqp.Channel).IsClosed() {
		err = value1.(*amqp.Channel).Close()
		if err != nil {
			return
		}
	}
	op.client.channelCache.Delete(publishQueue)
	op.client.pushConfirmListener.Delete(publishQueue)

	consumeQueue := queueName + "_queue:consume"
	value2, ok2 := op.client.channelCache.Load(consumeQueue)
	if ok2 && !value2.(*amqp.Channel).IsClosed() {
		err = value2.(*amqp.Channel).Close()
		if err != nil {
			return
		}
	}
	op.client.channelCache.Delete(consumeQueue)
	op.client.chCloseListener.Delete(consumeQueue)

	return
}

func (op *RabbitMQOperator) checkCommonChannel() (err error) {
	for i := 0; i < RetryTimes; i++ {
		err = op.checkCommonChannelCore()
		if err == nil {
			break
		}
	}
	return
}

func (op *RabbitMQOperator) checkCommonChannelCore() (err error) {
	if !op.client.commonCh.IsClosed() {
		return
	}

	if op.client.conn.IsClosed() {
		time.Sleep(1 * time.Second)
		if op.client.conn.IsClosed() {
			err = errors.New("rabbitmq is disconnected")
			return
		}
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	op.client.commonCh, err = op.client.conn.Channel()
	return
}

func (op *RabbitMQOperator) GetMessageCount(queueName string) (count int, err error) {
	err = op.checkCommonChannel()
	if err != nil {
		return
	}

	queue, err := op.client.commonCh.QueueInspect(queueName)
	if err != nil {
		return
	}
	count = queue.Messages
	return
}

func (op *RabbitMQOperator) Close() (err error) {
	op.mu.Lock()
	defer op.mu.Unlock()
	if atomic.LoadInt32(&op.closed) == Closed {
		return
	}

	if !op.client.conn.IsClosed() {
		err = op.client.conn.Close()
		if err != nil {
			return
		}
	}

	atomic.StoreInt32(&op.closed, Closed)
	close(op.closeCh)
	op.isReady = false
	return
}

func (op *RabbitMQOperator) Ack(ctx context.Context, msg amqp.Delivery) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", msg.MessageId))
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
	case <-op.closeCh:
		logger.Info("[Consume]mq closed, ack cancel for: " + msg.RoutingKey)
		return
	case done := <-ch:
		if done {
			logger.Info("[Ack]ack done")
		} else {
			logger.Info("[Ack]ack failed")
		}
	case <-time.After(DefaultRetryWaitTimes * time.Second):
		logger.Info("[Ack]ack timeout")
	}
}

func (op *RabbitMQOperator) Nack(ctx context.Context, msg amqp.Delivery) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", msg.MessageId))
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
	case <-op.closeCh:
		logger.Info("[Consume]mq closed, nack cancel for: " + msg.RoutingKey)
		return
	case done := <-ch:
		if done {
			logger.Info("[Nack]Nack done")
		} else {
			logger.Info("[Nack]Nack failed")
		}
		close(ch)
	case <-time.After(DefaultRetryWaitTimes * time.Second):
		logger.Info("[Nack]Nack timeout")
	}
}
