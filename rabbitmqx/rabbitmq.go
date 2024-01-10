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
	"github.com/jasonlabz/potato/core/times"
	log "github.com/jasonlabz/potato/log/zapx"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DefaultRetryTimes        = 5 * time.Second
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

// MQConfig 定义队列连接信息
type MQConfig struct {
	UserName string `json:"user_name"` // 用户
	Password string `json:"password"`  // 密码
	Host     string `json:"host"`      // 服务地址
	Port     int    `json:"port"`      // 端口
}

func (c *MQConfig) Validate() error {
	if c.UserName == "" {
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
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", c.UserName, c.Password, c.Host, c.Port)
}

func NewRabbitMQ(ctx context.Context, config *MQConfig) (op *RabbitOperator, err error) {
	logger := log.GetLogger(ctx)
	op = &RabbitOperator{}
	// init
	op.client = &Client{
		closeConnNotify: make(chan *amqp.Error, 1),
	}
	op.closeCh = make(chan bool)
	op.notifyAllChan = make(chan bool)
	op.name = fmt.Sprintf("rmq_%v", rand.Int31())

	// validate config param
	if err = config.Validate(); err != nil {
		logger.Error(fmt.Sprintf("init rabbitmq client error: %v", err))
		return
	}
	op.config = config

	// ready connect rmq
	timer := time.NewTimer(DefaultRetryTimes)
	defer timer.Stop()
	for {
		op.client.conn, err = amqp.DialTLS(config.Addr(), &tls.Config{InsecureSkipVerify: true})
		if err == nil {
			op.isReady = true
			op.client.commonCh, err = op.client.conn.Channel()
			logger.Info(fmt.Sprintf("rabbitmq init success[addr:%s]", config.Addr()))
			break
		}
		<-timer.C
		logger.Warn(fmt.Sprintf("wait %ss for retry to connect...", DefaultRetryTimes))
	}
	// a new goroutine for check disconnect
	go op.tryReConnect(true)

	return
}

type RabbitOperator struct {
	name          string
	config        *MQConfig
	client        *Client
	closeCh       chan bool
	notifyAllChan chan bool
	isReady       bool
	closed        int32
	mu            sync.Mutex
}

type Client struct {
	conn            *amqp.Connection
	commonCh        *amqp.Channel
	channelCache    sync.Map
	cacheMu         sync.Mutex
	closeConnNotify chan *amqp.Error
}

func (op *RabbitOperator) SetLogger(logger amqp.Logging) {
	amqp.Logger = logger
}

func (op *RabbitOperator) tryReConnect(daemon bool) (connected bool) {
	logger := log.GetLogger(context.Background())
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

	timer := time.NewTimer(DefaultRetryTimes)
	defer timer.Stop()

	for {
		if !op.isReady && atomic.LoadInt32(&op.closed) != Closed {
			conn, err := amqp.DialTLS(op.config.Addr(), &tls.Config{InsecureSkipVerify: true})
			if err == nil {
				op.client.conn = conn
				op.client.conn.NotifyClose(op.client.closeConnNotify)
				op.isReady = true
				logger.Info(fmt.Sprintf("rabbitmq reconnect success[addr:%s]", op.config.Addr()))
				continue
			}
			select {
			case s := <-sig:
				logger.Info(fmt.Sprintf("recived： %v, exiting... ", s))
				return
			case <-op.closeCh:
				logger.Info(fmt.Sprintf("rabbitmq is closed, exiting..."))
				return
			case <-timer.C:
			}
		}

		if !daemon {
			connected = true
			break
		}

		select {
		case s := <-sig:
			logger.Info(fmt.Sprintf("recived： %v, exiting... ", s))
			return
		case <-op.closeCh:
			logger.Info(fmt.Sprintf("rabbitmq is closed, exiting..."))
			return
		case <-op.client.closeConnNotify:
			op.isReady = false
			op.client.channelCache = sync.Map{}
			// notify all channel retry init
			close(op.notifyAllChan)
			op.notifyAllChan = make(chan bool)
			logger.Error(fmt.Sprintf("rabbitmq is disconnect, retrying..."))
		case <-timer.C:
		}
	}
	return
}

func (op *RabbitOperator) getChannelForExchange(exchange string) (channel *amqp.Channel, err error) {
	exchange = exchange + "_exchange:publish"
	ch, ok := op.client.channelCache.Load(exchange)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()

	ch, ok = op.client.channelCache.Load(exchange)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}
	if op.client.conn.IsClosed() {
		op.mu.Lock()
		defer op.mu.Unlock()
		if op.client.conn.IsClosed() {
			op.isReady = false
			op.client.channelCache = sync.Map{}
			if connected := op.tryReConnect(false); !connected {
				err = errors.New("rabbitmq is closed")
				return
			}
		}
	}
	channel, err = op.client.conn.Channel()
	if err != nil {
		return
	}
	op.client.channelCache.Store(exchange, channel)
	return
}

func (op *RabbitOperator) getChannelForQueue(isConsume bool, queue string) (channel *amqp.Channel, err error) {
	if isConsume {
		queue += "_queue:consume"
	} else {
		queue += "_queue:publish"
	}
	ch, ok := op.client.channelCache.Load(queue)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()

	ch, ok = op.client.channelCache.Load(queue)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}
	if op.client.conn.IsClosed() {
		op.mu.Lock()
		defer op.mu.Unlock()
		if op.client.conn.IsClosed() {
			op.isReady = false
			op.client.channelCache = sync.Map{}
			if connected := op.tryReConnect(false); !connected {
				err = errors.New("rabbitmq is closed")
				return
			}
		}
	}
	channel, err = op.client.conn.Channel()
	if err != nil {
		return
	}
	//notifyPush := make(chan amqp.Confirmation, 1)
	//channel.NotifyPublish(notifyPush)
	//
	//go func() {
	//
	//}()
	op.client.channelCache.Store(queue, channel)
	return
}

func (op *RabbitOperator) getChannel(isConsume bool, exchange, queue string) (channel *amqp.Channel, err error) {
	if isConsume {
		channel, err = op.getChannelForQueue(isConsume, queue)
		return
	}
	if exchange != "" {
		channel, err = op.getChannelForExchange(exchange)
		return
	}
	channel, err = op.getChannelForQueue(false, queue)
	return
}

func (op *RabbitOperator) PushDelayMessage(ctx context.Context, body *PushDelayBody) (err error) {
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	timer := time.NewTimer(DefaultRetryTimes)
	defer timer.Stop()

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
		err = op.pushDelayMessageCore(ctx, body)
		if err != nil {
			logger.WithError(err).Error(fmt.Sprintf("[push] Push failed. after %d seconds and retry...", DefaultRetryTimes))
			select {
			case <-op.closeCh:
				logger.Error(fmt.Sprintf("[push]rmq closed, push msg cancel"))
				err = errors.New("rabbitmq connection closed")
				return
			case <-timer.C:
			}
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg: " + string(body.Body))
	return
}
func (op *RabbitOperator) pushDelayMessageCore(ctx context.Context, body *PushDelayBody) (err error) {
	delayStr := strconv.FormatInt(int64(body.DelayTime), 10)
	delayQueueName := body.QueueName + "_delay:" + delayStr
	delayRouteKey := body.RoutingKey + "_delay:" + delayStr

	channel, err := op.getChannelForQueue(false, body.QueueName)
	if err != nil {
		return
	}

	if body.OpenConfirm {
		err = channel.Confirm(body.ConfirmNoWait)
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
	err = channel.ExchangeDeclare(body.ExchangeName, string(body.ExchangeType), true, false, false, true, body.ExchangeArgs)
	if err != nil {
		return
	}

	// 队列不存在,声明队列
	// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
	// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
	_, err = channel.QueueDeclare(body.QueueName, true, false, false, true, body.Args)
	if err != nil {
		return
	}

	// 定义延迟队列(死信队列)
	_, err = channel.QueueDeclare(delayQueueName, true, false, false, true,
		amqp.Table{
			"x-dead-letter-exchange": body.ExchangeName, // 指定死信交换机
			//"x-dead-letter-routing-key": body.RoutingKey,   // 指定死信routing-key
		})
	// 延迟队列绑定到exchange
	err = channel.QueueBind(body.QueueName, body.RoutingKey, body.ExchangeName, true, nil)
	if err != nil {
		return err
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
	err = channel.PublishWithContext(ctx, "", delayRouteKey, false, false, publishingMsg)
	if err != nil {
		return
	}
	return
}

func (op *RabbitOperator) PushExchange(ctx context.Context, body *ExchangePushBody) (err error) {
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	timer := time.NewTimer(DefaultRetryTimes)
	defer timer.Stop()
	body.Validate()

	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		err = op.pushExchangeCore(ctx, body)
		if err != nil {
			logger.WithError(err).Error(fmt.Sprintf("[push] Push failed. after %d seconds and retry...", DefaultRetryTimes))
			select {
			case <-op.closeCh:
				logger.Error(fmt.Sprintf("[push]rmq closed, push msg cancel"))
				err = errors.New("rabbitmq connection closed")
				return
			case <-timer.C:
			}
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg: " + string(body.Body))
	return
}

func (op *RabbitOperator) PushQueue(ctx context.Context, body *QueuePushBody) (err error) {
	if body.MessageId == "" {
		body.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", body.MessageId))
	timer := time.NewTimer(DefaultRetryTimes)
	defer timer.Stop()

	body.Validate()
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		err = op.pushQueueCore(ctx, body)
		if err != nil {
			logger.WithError(err).Error(fmt.Sprintf("[push] Push failed. after %d seconds and retry...", DefaultRetryTimes))
			select {
			case <-op.closeCh:
				logger.Error(fmt.Sprintf("[push]mq closed, push msg cancel"))
				err = errors.New("rabbitmq connection closed")
				return
			case <-timer.C:
			}
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg: " + string(body.Body))
	return
}

func (op *RabbitOperator) Push(ctx context.Context, msg *PushBody) (err error) {
	if msg.MessageId == "" {
		msg.MessageId = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", msg.MessageId))
	timer := time.NewTimer(DefaultRetryTimes)
	defer timer.Stop()
	for i := 0; i < RetryTimes; i++ {
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Error("rabbitmq connection is closed, push cancel")
			err = errors.New("connection is closed")
			return
		}
		err = op.pushCore(ctx, msg)
		if err != nil {
			select {
			case <-op.closeCh:
				logger.Error(fmt.Sprintf("[push]mq closed, push msg cancel"))
				err = errors.New("rabbitmq connection closed")
				return
			case <-timer.C:
			}
			logger.Error(fmt.Sprintf("[push] Push failed. Retrying..."))
			continue
		}
		logger.Info("[push] Push msg success.")
		return
	}
	logger.Info("[push] Push msg failed. msg: " + string(msg.Body))
	return
}

/*
@method: pushQueueCore
@arg: QueuePushBody ->  Args是队列的参数设置，例如优先级队列为amqp.Table{"x-max-priority":10}
@description: 向队列推送消息
*/
func (op *RabbitOperator) pushQueueCore(ctx context.Context, body *QueuePushBody) (err error) {
	channel, err := op.getChannelForQueue(false, body.QueueName)
	if err != nil {
		return
	}

	if body.OpenConfirm {
		err = channel.Confirm(body.ConfirmNoWait)
		if err != nil {
			return
		}
	}

	// 队列不存在,声明队列
	// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
	// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
	_, err = channel.QueueDeclare(body.QueueName, true, false, false, true, body.Args)
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
	return
}

/*
@method: pushExchangeCore
@arg: ExchangePushBody ->  Args是交换机和队列的参数设置，例如优先级队列为amqp.Table{"x-max-priority":10}
@description: 向队列推送消息
*/
func (op *RabbitOperator) pushExchangeCore(ctx context.Context, body *ExchangePushBody) (err error) {
	channel, err := op.getChannelForExchange(body.ExchangeName)
	if err != nil {
		return
	}

	if body.OpenConfirm {
		err = channel.Confirm(body.ConfirmNoWait)
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
	err = channel.ExchangeDeclare(body.ExchangeName, string(body.ExchangeType), true, false, false, true, body.ExchangeArgs)
	if err != nil {
		return
	}

	// 交换机绑定队列处理
	for queue, bindingKey := range body.BindingKeyMap {
		table := body.QueueArgs[queue]
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = channel.QueueDeclare(queue, true, false, false, true, table)
		if err != nil {
			return
		}

		// 队列绑定
		err = channel.QueueBind(queue, bindingKey, body.ExchangeName, true, nil)
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
	return
}

func (op *RabbitOperator) pushCore(ctx context.Context, msg *PushBody) (err error) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", msg.MessageId))
	channel, err := op.getChannel(false, msg.ExchangeName, msg.QueueName)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if msg.OpenConfirm {
		err = channel.Confirm(msg.ConfirmNoWait)
		if err != nil {
			logger.Error(err.Error())
			return
		}
	}

	msg.Validate()

	if msg.ExchangeName != "" {
		// 注册交换机
		// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
		// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
		err = channel.ExchangeDeclare(msg.ExchangeName, string(msg.ExchangeType), true, false, false, true, msg.ExchangeArgs)
		if err != nil {
			logger.Error(fmt.Sprintf("MQ failed to declare the exchange:%s \n", err))
			return
		}

		// 交换机绑定队列处理
		for queue, bindingKey := range msg.BindingKeyMap {
			table := msg.QueueArgs[queue]
			// 队列不存在,声明队列
			// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
			// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
			_, err = channel.QueueDeclare(queue, true, false, false, true, table)
			if err != nil {
				logger.Error(fmt.Sprintf("MQ declare queue failed:%s \n", err))
				return
			}

			// 队列绑定
			err = channel.QueueBind(queue, bindingKey, msg.ExchangeName, true, nil)
			if err != nil {
				logger.Error(fmt.Sprintf("MQ binding queue failed:%s \n", err))
				return
			}
		}
	}

	if msg.QueueName != "" {
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = channel.QueueDeclare(msg.QueueName, true, false, false, true, msg.Args)
		if err != nil {
			logger.Error(fmt.Sprintf("MQ declare queue failed:%s \n", err))
			return
		}
	}

	publishingMsg := amqp.Publishing{
		Timestamp:       times.Now(),
		Body:            msg.Body,
		Headers:         msg.Headers,
		ContentEncoding: msg.ContentEncoding,
		ReplyTo:         msg.ReplyTo,
		Expiration:      msg.Expiration,
		MessageId:       msg.MessageId,
		Type:            msg.Type,
		UserId:          msg.UserId,
		AppId:           msg.AppId,
	}
	if msg.Priority > 0 {
		publishingMsg.Priority = msg.Priority
	}
	if msg.DeliveryMode == 0 {
		publishingMsg.DeliveryMode = amqp.Persistent
	}
	if msg.ContentType == "" {
		publishingMsg.ContentType = "text/plain"
	}
	// 发送任务消息
	err = channel.PublishWithContext(ctx, msg.ExchangeName, msg.RoutingKey, false, false, publishingMsg)
	if err != nil {
		logger.Error(fmt.Sprintf("MQ task failed to be sent:%s \n", err))
		return
	}
	return
}

func (op *RabbitOperator) Consume(ctx context.Context, param *ConsumeBody) (contents chan amqp.Delivery, err error) {
	logger := log.GetLogger(ctx)
	if !op.isReady {
		logger.Error("rabbitmq connection is not ready, consume cancel")
		err = errors.New("connection is not ready")
		return
	}
	contents = make(chan amqp.Delivery, 3)

	go func() {
		rLogger := log.GetLogger(context.Background())
		defer func() {
			if e := recover(); e != nil {
				logger.Error(fmt.Sprintf("recover_panic: %v", e))
			}
		}()

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)

		timer := time.NewTimer(DefaultConsumeRetryTimes)
		defer timer.Stop()
	process:
		resChan, innerErr := op.consumeCore(ctx, param)
		// Keep retrying when consumption fails
		for innerErr != nil {
			select {
			case s := <-sig:
				rLogger.Info(fmt.Sprintf("[consume:%s]recived： %v, exiting... ", param.QueueName, s))
				return
			case <-op.closeCh:
				rLogger.Info(fmt.Sprintf("[consume:%s]rabbitmq is closed, exiting...", param.QueueName))
				return
			case <-timer.C:
				rLogger.Error(fmt.Sprintf("[consume:%s]consume msg error: %s, retrying ...", param.QueueName, innerErr.Error()))
			}
			resChan, innerErr = op.consumeCore(ctx, param)
		}

		// Circular consumption of data
		allChan := op.notifyAllChan
		for {
			select {
			case s := <-sig:
				rLogger.Info(fmt.Sprintf("[consume:%s]recived： %v, exiting... ", param.QueueName, s))
				return
			case <-op.closeCh:
				rLogger.Info(fmt.Sprintf("[consume:%s]rabbitmq is closed, exiting...", param.QueueName))
				return
			case <-allChan:
				allChan = op.notifyAllChan
				rLogger.Warn(fmt.Sprintf("[consume:%s]rabbitmq is reconnected, reconsume...", param.QueueName))
				goto process
			case item := <-resChan:
				if len(item.Body) == 0 {
					continue
				}
				select {
				case s := <-sig:
					rLogger.Info(fmt.Sprintf("[consume:%s]recived： %v, exiting... ", param.QueueName, s))
					return
				case <-op.closeCh:
					rLogger.Info(fmt.Sprintf("[consume:%s]rabbitmq is closed, exiting...", param.QueueName))
					return
				case <-allChan:
					allChan = op.notifyAllChan
					rLogger.Warn(fmt.Sprintf("[consume:%s]rabbitmq is reconnected, reconsume...", param.QueueName))
					goto process
				case contents <- item:
					rLogger.Info(fmt.Sprintf("[consume:%s]recived msg success: msg_id --> %s", param.QueueName, item.MessageId))
				}
			case <-timer.C:
				continue
			}
		}

	}()
	return
}

func (op *RabbitOperator) consumeCore(ctx context.Context, param *ConsumeBody) (contents <-chan amqp.Delivery, err error) {
	logger := log.GetLogger(ctx)
	if !op.isReady {
		logger.Error("rabbitmq connection is not ready, push cancel")
		err = errors.New("connection is not ready")
		return
	}
	channel, err := op.getChannel(true, param.ExchangeName, param.QueueName)
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
	_, err = channel.QueueDeclare(param.QueueName, true, false, false, true, param.QueueArgs)
	if err != nil {
		logger.Error(fmt.Sprintf("MQ declare queue failed:%s \n", err))
		return
	}
	if param.ExchangeName != "" {
		// 绑定任务
		err = channel.QueueBind(param.QueueName, param.RoutingKey, param.ExchangeName, true, nil)
		if err != nil {
			logger.Error(fmt.Sprintf("binding queue failed:%s \n", err))
			return
		}
	}

	// 获取消费通道,确保rabbitMQ一个一个发送消息
	err = channel.Qos(param.FetchCount, 0, true)
	if err != nil {
		logger.Error(fmt.Sprintf("open qos error:%s \n", err))
		return
	}
	var args amqp.Table
	if param.XPriority != 0 {
		args = amqp.Table{"x-priority": param.XPriority}
	}
	contents, err = channel.Consume(param.QueueName, "", false, false, false, false, args)
	if err != nil {
		logger.Error(fmt.Sprintf("The acquisition of the consumption channel is abnormal:%s \n", err))
		return
	}
	return
}

func (op *RabbitOperator) releaseExchangeChannel(exchangeName string) (err error) {
	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()
	exchangeName += "_exchange:publish"
	value, ok := op.client.channelCache.Load(exchangeName)
	if ok && !value.(*amqp.Channel).IsClosed() {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
		op.client.channelCache.Delete(exchangeName)
	}
	return
}

func (op *RabbitOperator) DeclareExchange(exchangeName, exchangeType string, args amqp.Table) (err error) {
	channel, err := op.getCommonChannel()
	if err != nil {
		return
	}

	err = channel.ExchangeDeclare(exchangeName, exchangeType, true, false, false, false, args)
	if err != nil {
		return err
	}
	return
}

func (op *RabbitOperator) DeclareQueue(queueName string, args amqp.Table) (err error) {
	channel, err := op.getCommonChannel()
	if err != nil {
		return
	}

	_, err = channel.QueueDeclare(queueName, true, false, false, true, args)
	if err != nil {
		return err
	}
	return
}

func (op *RabbitOperator) BindQueue(exchangeName, queueName, routingKey string, args amqp.Table) (err error) {
	channel, err := op.getCommonChannel()
	if err != nil {
		return
	}

	err = channel.QueueBind(queueName, routingKey, exchangeName, false, args)
	if err != nil {
		return err
	}
	return
}

func (op *RabbitOperator) UnBindQueue(exchangeName, queueName, routingKey string, args amqp.Table) (err error) {
	channel, err := op.getCommonChannel()
	if err != nil {
		return
	}

	err = channel.QueueUnbind(queueName, routingKey, exchangeName, args)
	if err != nil {
		return err
	}
	return
}

func (op *RabbitOperator) DeleteExchange(exchangeName string) (err error) {
	channel, err := op.getCommonChannel()
	if err != nil {
		return
	}

	err = channel.ExchangeDelete(exchangeName, false, false)
	if err != nil {
		return err
	}
	go func() {
		_ = op.releaseExchangeChannel(exchangeName)
	}()
	return
}

func (op *RabbitOperator) DeleteQueue(queueName string) (err error) {
	channel, err := op.getCommonChannel()
	if err != nil {
		return
	}

	_, err = channel.QueueDelete(queueName, false, false, false)
	if err != nil {
		return err
	}
	go func() {
		_ = op.releaseQueueChannel(queueName)
	}()
	return
}

func (op *RabbitOperator) releaseQueueChannel(queueName string) (err error) {
	op.client.cacheMu.Lock()
	defer op.client.cacheMu.Unlock()
	value, ok := op.client.channelCache.Load(queueName + "_queue:publish")
	if ok {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
		op.client.channelCache.Delete(queueName + "_queue:publish")
	}
	value, ok = op.client.channelCache.Load(queueName + "_queue:consume")
	if ok {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
		op.client.channelCache.Delete(queueName + "__consume__")
	}
	return
}

func (op *RabbitOperator) getCommonChannel() (channel *amqp.Channel, err error) {
	if op.client.commonCh.IsClosed() {
		op.mu.Lock()
		defer op.mu.Unlock()
		if op.client.commonCh == nil || op.client.commonCh.IsClosed() {
			op.client.commonCh, err = op.client.conn.Channel()
		}
	}
	channel = op.client.commonCh
	return
}

func (op *RabbitOperator) GetMessageCount(queueName string) (count int, err error) {
	channel, err := op.getCommonChannel()
	if err != nil {
		return
	}

	queue, err := channel.QueueInspect(queueName)
	if err != nil {
		return
	}
	count = queue.Messages
	return
}

func (op *RabbitOperator) Close() (err error) {
	op.mu.Lock()
	defer op.mu.Unlock()
	if atomic.LoadInt32(&op.closed) == Closed {
		return
	}

	err = op.client.conn.Close()
	if err != nil {
		return err
	}

	atomic.StoreInt32(&op.closed, Closed)
	close(op.closeCh)
	close(op.notifyAllChan)

	op.isReady = false
	return
}

func (op *RabbitOperator) Ack(ctx context.Context, msg *amqp.Delivery) {
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
	case <-time.After(DefaultRetryTimes * time.Second):
		logger.Info("[Ack]ack timeout")
	}
}

func (op *RabbitOperator) Nack(ctx context.Context, msg *amqp.Delivery) {
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
	case <-time.After(DefaultRetryTimes * time.Second):
		logger.Info("[Nack]Nack timeout")
	}
}
