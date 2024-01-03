package rabbitmqx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
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
	queueCh         sync.Map
	exchangeMu      sync.Mutex
	exchangeCh      sync.Map
	queueMu         sync.Mutex
	closeConnNotify chan *amqp.Error
	closeChanNotify chan *amqp.Error
	routingKeyQueue map[string]string // routingKeyMap
	exchangeName    string            // 交换机名称
	exchangeType    ExchangeType      // 交换机类型
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
		if atomic.LoadInt32(&op.closed) == Closed {
			logger.Info(fmt.Sprintf("rabbitmq is closed [addr:%s], exit reconnect goroutime", op.config.Addr()))
			return
		}

		if !op.isReady {
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
			op.client.exchangeCh = sync.Map{}
			op.client.queueCh = sync.Map{}
			// notify all channel retry init
			close(op.notifyAllChan)
			op.notifyAllChan = make(chan bool)
			logger.Error(fmt.Sprintf("rabbitmq is disconnect, retrying..."))
		case <-timer.C:
		}
	}
	return
}

func (op *RabbitOperator) getChannelForExchange(isConsume bool, exchange string) (channel *amqp.Channel, err error) {
	ch, ok := op.client.exchangeCh.Load(exchange)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.exchangeMu.Lock()
	defer op.client.exchangeMu.Unlock()

	ch, ok = op.client.exchangeCh.Load(exchange)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}
	if op.client.conn.IsClosed() {
		op.mu.Lock()
		defer op.mu.Unlock()
		if op.client.conn.IsClosed() {
			op.isReady = false
			op.client.queueCh = sync.Map{}
			op.client.exchangeCh = sync.Map{}
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
	op.client.exchangeCh.Store(exchange, channel)
	return
}

func (op *RabbitOperator) getChannelForQueue(isConsume bool, queue string) (channel *amqp.Channel, err error) {
	if isConsume {
		queue += "__consume__"
	} else {
		queue += "__push__"
	}
	ch, ok := op.client.queueCh.Load(queue)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.queueMu.Lock()
	defer op.client.queueMu.Unlock()

	ch, ok = op.client.queueCh.Load(queue)
	if ok && !ch.(*amqp.Channel).IsClosed() {
		channel = ch.(*amqp.Channel)
		return
	}
	if op.client.conn.IsClosed() {
		op.mu.Lock()
		defer op.mu.Unlock()
		if op.client.conn.IsClosed() {
			op.isReady = false
			op.client.queueCh = sync.Map{}
			op.client.exchangeCh = sync.Map{}
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
	op.client.queueCh.Store(queue, channel)
	return
}

func (op *RabbitOperator) getChannel(isConsume bool, exchange, queue string) (channel *amqp.Channel, err error) {
	if isConsume {
		channel, err = op.getChannelForQueue(isConsume, queue)
		return
	}
	if exchange != "" {
		channel, err = op.getChannelForExchange(false, exchange)
		return
	}
	channel, err = op.getChannelForQueue(false, queue)
	return
}

type PushBody struct {
	ConfirmMode      bool
	NoWait           bool
	ExchangeName     string
	ExchangeType     ExchangeType
	BindingKeyMap    map[string]string
	RoutingKey       string
	QueueName        string
	QueueMaxPriority map[string]uint8
	XMaxPriority     uint8
	amqp.Publishing
}

func (p *PushBody) Validate() {
	if len(p.BindingKeyMap) == 0 {
		p.BindingKeyMap = map[string]string{}
	}

	if p.ExchangeName != "" {
		if string(p.ExchangeType) == "" {
			//  default exchangeType is fanout
			p.ExchangeType = ExchangeTypeFanout
		}

		if p.QueueName != "" {
			p.BindingKeyMap[p.QueueName] = p.QueueName
			p.QueueName = ""
		}
	}

	if p.ExchangeName == "" {
		p.BindingKeyMap = nil
	}

	// 限制优先级为 0~10
	if p.Priority > 10 {
		p.Priority = 10
	}

	if p.RoutingKey == "" {
		p.RoutingKey = p.QueueName
	}

	for queue, bindKey := range p.BindingKeyMap {
		if bindKey == "" {
			p.BindingKeyMap[queue] = queue
		}
	}

	pri, ok := p.QueueMaxPriority[p.QueueName]
	if ok {
		p.XMaxPriority = pri
	} else if p.XMaxPriority == 0 && len(p.QueueMaxPriority) > 0 {
		for _, u := range p.QueueMaxPriority {
			p.XMaxPriority = u
			break
		}
	}
}

func (p *PushBody) SetPriority(priority uint8) *PushBody {
	p.Priority = priority
	return p
}

func (p *PushBody) SetQueueWithMaxPriority(priority uint8, queues ...string) *PushBody {
	if p.XMaxPriority == 0 {
		p.XMaxPriority = priority
	}
	if p.QueueMaxPriority == nil {
		p.QueueMaxPriority = make(map[string]uint8)
	}
	for _, queue := range queues {
		p.QueueMaxPriority[queue] = priority
	}
	return p
}

func (p *PushBody) SetExchangeName(exchangeName string) *PushBody {
	p.ExchangeName = exchangeName
	return p
}

func (p *PushBody) SetExchangeType(exchangeType ExchangeType) *PushBody {
	p.ExchangeType = exchangeType
	return p
}

func (p *PushBody) BindQueue(queueName, bindingKey string) *PushBody {
	if len(p.BindingKeyMap) == 0 {
		p.BindingKeyMap = map[string]string{}
	}
	if bindingKey == "" {
		bindingKey = queueName
	}
	p.BindingKeyMap[queueName] = bindingKey
	return p
}

func (p *PushBody) SetRoutingKey(routingKey string) *PushBody {
	p.RoutingKey = routingKey
	return p
}

func (p *PushBody) SetQueueName(queueName string) *PushBody {
	p.QueueName = queueName
	return p
}

func (p *PushBody) SetConfirmMode(confirm, noWait bool) *PushBody {
	p.ConfirmMode = confirm
	p.NoWait = noWait
	return p
}

func (p *PushBody) SetDeliveryMode(deliveryMode uint8) *PushBody {
	p.DeliveryMode = deliveryMode
	return p
}

func (p *PushBody) SetMsg(msg []byte) *PushBody {
	p.Body = msg
	return p
}

type ArgOption func(a amqp.Table)

func WithMaxPriority(priority int) ArgOption {
	return func(a amqp.Table) {
		a["x-max-priority"] = priority
	}
}

func WithMsgTTL(duration time.Duration) ArgOption {
	return func(a amqp.Table) {
		a["x-message-ttl"] = duration
	}
}

func WithDeadLetterExchange(exchange, routingKey string) ArgOption {
	return func(a amqp.Table) {
		a["x-dead-letter-exchange"] = exchange
		a["x-dead-letter-routing-key"] = routingKey
	}
}

func (op *RabbitOperator) Push(ctx context.Context, msg *PushBody, args ...ArgOption) (err error) {
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
		err = op.pushCore(ctx, msg, args...)
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

func (op *RabbitOperator) pushCore(ctx context.Context, msg *PushBody, args ...ArgOption) (err error) {
	logger := log.GetLogger(ctx).WithField(log.String("msg_id", msg.MessageId))
	channel, err := op.getChannel(false, msg.ExchangeName, msg.QueueName)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if msg.ConfirmMode {
		err = channel.Confirm(msg.NoWait)
		if err != nil {
			logger.Error(err.Error())
			return
		}
	}

	msg.Validate()

	table := amqp.Table{}
	for _, arg := range args {
		arg(table)
	}
	if msg.ExchangeName != "" {
		// 注册交换机
		// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
		// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
		err = channel.ExchangeDeclare(msg.ExchangeName, string(msg.ExchangeType), true, false, false, true, nil)
		if err != nil {
			logger.Error(fmt.Sprintf("MQ failed to declare the exchange:%s \n", err))
			return
		}

		// 交换机绑定队列处理
		for queue, bindingKey := range msg.BindingKeyMap {
			xMaxPri, ok := msg.QueueMaxPriority[queue]
			if ok {
				table["x-max-priority"] = xMaxPri
			}
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
		xMaxPri, ok := msg.QueueMaxPriority[msg.QueueName]
		if ok {
			table["x-max-priority"] = xMaxPri
		}
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = channel.QueueDeclare(msg.QueueName, true, false, false, true, table)
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

type ConsumeBody struct {
	ExchangeName string
	RoutingKey   string
	QueueName    string
	XMaxPriority uint8
	XPriority    int
	FetchCount   int
}

func (op *RabbitOperator) Consume(ctx context.Context, param *ConsumeBody, options ...ArgOption) (contents chan amqp.Delivery, err error) {
	logger := log.GetLogger(ctx)
	if !op.isReady {
		logger.Error("rabbitmq connection is not ready, consume cancel")
		err = errors.New("connection is not ready")
		return
	}
	contents = make(chan amqp.Delivery, 3)
	table := amqp.Table{}
	for _, opt := range options {
		opt(table)
	}

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
		resChan, innerErr := op.consumeCore(ctx, param, table)
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
			resChan, innerErr = op.consumeCore(ctx, param, table)
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

func (op *RabbitOperator) consumeCore(ctx context.Context, param *ConsumeBody, table amqp.Table) (contents <-chan amqp.Delivery, err error) {
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
	if param.XMaxPriority > 0 {
		table["x-max-priority"] = param.XMaxPriority
	}
	// 队列不存在,声明队列
	// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
	// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
	_, err = channel.QueueDeclare(param.QueueName, true, false, false, true, table)
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

func (op *RabbitOperator) ReleaseExchangeChannel(exchangeName string) (err error) {
	op.client.exchangeMu.Lock()
	defer op.client.exchangeMu.Unlock()
	value, ok := op.client.exchangeCh.Load(exchangeName)
	if ok && !value.(*amqp.Channel).IsClosed() {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
		op.client.exchangeCh.Delete(exchangeName)
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

	return
}

func (op *RabbitOperator) ReleaseQueueChannel(queueName string) (err error) {
	op.client.queueMu.Lock()
	defer op.client.queueMu.Unlock()
	value, ok := op.client.queueCh.Load(queueName + "__push__")
	if ok {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
		op.client.queueCh.Delete(queueName + "__push__")
	}
	value, ok = op.client.queueCh.Load(queueName + "__consume__")
	if ok {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
		op.client.queueCh.Delete(queueName + "__consume__")
	}
	return
}

func (op *RabbitOperator) getCommonChannel() (channel *amqp.Channel, err error) {
	if op.client.commonCh.IsClosed() {
		op.mu.Lock()
		defer op.mu.Unlock()
		if op.client.commonCh.IsClosed() {
			op.client.commonCh, err = op.client.conn.Channel()
		}
	}
	channel = op.client.commonCh
	return
}

func (op *RabbitOperator) GetCount(ctx context.Context, queueName string) (count int, err error) {
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
