package rabbitmqx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/jasonlabz/potato/core/times"
	"github.com/jasonlabz/potato/log"
	amqp "github.com/rabbitmq/amqp091-go"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	DefaultRetryTimes = 5 * time.Second
	Closed            = 1
)

type ModeType string

const (
	ModeTypeQueue  ModeType = "queue"   // 队列模式
	ModeTypePubSub ModeType = "pub_sub" // 发布/订阅模式
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

// QueueExchange 定义队列交换机对象
type QueueExchange struct {
	QueueName    string `json:"queue_name"`    // 队列名称
	RoutingKey   string `json:"routing_key"`   // key值
	ExchangeName string `json:"exchange_name"` // 交换机名称
	ExchangeType string `json:"exchange_type"` // 交换机类型
}

type RabbitOperator struct {
	name    string
	config  *MQConfig
	client  *RabbitMQClient
	closeCh chan bool
	isReady bool
	closed  int32
	mu      sync.Mutex
}

type RabbitMQClient struct {
	mode            ModeType // 模式类型，默认queue模式
	conn            *amqp.Connection
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

func (op *RabbitOperator) InitRabbitMQ(ctx context.Context, config *MQConfig) (err error) {
	logger := log.GetCurrentLogger(ctx)
	op.mu.Lock()
	defer op.mu.Unlock()
	if op.isReady {
		// rabbitmq already init success
		return
	}
	// init
	op.client = &RabbitMQClient{
		closeConnNotify: make(chan *amqp.Error, 1),
	}
	op.closeCh = make(chan bool)
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
	go op.tryReConnect()

	return
}

func (op *RabbitOperator) SetLogger(logger amqp.Logging) {
	amqp.Logger = logger
}

func (op *RabbitOperator) tryReConnect() {
	logger := log.GetCurrentGormLogger(context.Background())
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
				op.isReady = true
				op.client.conn = conn
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

		select {
		case s := <-sig:
			logger.Info(fmt.Sprintf("recived： %v, exiting... ", s))
			return
		case <-op.closeCh:
			logger.Info(fmt.Sprintf("rabbitmq is closed, exiting..."))
			return
		case <-op.client.closeConnNotify:
			op.isReady = false
			logger.Error(fmt.Sprintf("rabbitmq is disconnect, retrying..."))
		case <-timer.C:

		}
	}
}

func (op *RabbitOperator) getChannelForExchange(ctx context.Context, exchange string) (channel *amqp.Channel, err error) {
	logger := log.GetCurrentLogger(ctx)
	ch, ok := op.client.exchangeCh.Load(exchange)
	if ok {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.exchangeMu.Lock()
	defer op.client.exchangeMu.Unlock()

	ch, ok = op.client.exchangeCh.Load(exchange)
	if ok {
		channel = ch.(*amqp.Channel)
		return
	}
	channel, err = op.client.conn.Channel()
	if err != nil {
		logger.Error(err.Error())
		return
	}
	op.client.exchangeCh.Store(exchange, channel)
	return
}

func (op *RabbitOperator) getChannelForQueue(ctx context.Context, isConsume bool, queue string) (channel *amqp.Channel, err error) {
	logger := log.GetCurrentLogger(ctx)
	if isConsume {
		queue += "__consume__"
	} else {
		queue += "__push__"
	}
	ch, ok := op.client.queueCh.Load(queue)
	if ok {
		channel = ch.(*amqp.Channel)
		return
	}

	op.client.queueMu.Lock()
	defer op.client.queueMu.Unlock()

	ch, ok = op.client.queueCh.Load(queue)
	if ok {
		channel = ch.(*amqp.Channel)
		return
	}
	channel, err = op.client.conn.Channel()
	if err != nil {
		logger.Error(err.Error())
		return
	}
	op.client.queueCh.Store(queue, channel)
	return
}

func (op *RabbitOperator) getChannel(ctx context.Context, isConsume bool, exchange, queue string) (channel *amqp.Channel, err error) {
	if exchange != "" {
		channel, err = op.getChannelForExchange(ctx, exchange)
		return
	}
	channel, err = op.getChannelForQueue(ctx, isConsume, queue)
	return
}

type PushMsg struct {
	exchangeName  string
	exchangeType  ExchangeType
	bindingKeyMap map[string]string
	routingKey    string
	queueName     string
	priority      uint8
	content       []byte
}

func (p *PushMsg) Validate() {
	if len(p.bindingKeyMap) == 0 {
		p.bindingKeyMap = map[string]string{}
	}

	if p.exchangeName != "" {
		if string(p.exchangeType) == "" {
			//  default exchangeType is fanout
			p.exchangeType = ExchangeTypeFanout
		}

		if p.queueName != "" {
			p.bindingKeyMap[p.queueName] = p.queueName
			p.queueName = ""
		}
	}

	if p.exchangeName == "" {
		p.bindingKeyMap = nil
	}

	// 限制优先级为 0~10
	if p.priority > 10 {
		p.priority = 10
	}
}

func (p *PushMsg) Priority(priority uint8) *PushMsg {
	p.priority = priority
	return p
}

func (p *PushMsg) ExchangeName(exchangeName string) *PushMsg {
	p.exchangeName = exchangeName
	return p
}

func (p *PushMsg) ExchangeType(exchangeType ExchangeType) *PushMsg {
	p.exchangeType = exchangeType
	return p
}

func (p *PushMsg) BindQueue(queueName, bindingKey string) *PushMsg {
	if len(p.bindingKeyMap) == 0 {
		p.bindingKeyMap = map[string]string{}
	}
	if bindingKey == "" {
		bindingKey = queueName
	}
	p.bindingKeyMap[queueName] = bindingKey
	return p
}

func (p *PushMsg) RoutingKey(routingKey string) *PushMsg {
	p.routingKey = routingKey
	return p
}

func (p *PushMsg) QueueName(queueName string) *PushMsg {
	p.queueName = queueName
	return p
}

func (p *PushMsg) Msg(msg []byte) *PushMsg {
	p.content = msg
	return p
}

type ArgOption func(a amqp.Table)

func WithMaxPriority(priority int) ArgOption {
	if priority > 10 {
		priority = 10
	}
	return func(a amqp.Table) {
		a["x-max-priority"] = priority
	}
}

func (op *RabbitOperator) PushMessage(ctx context.Context, msg *PushMsg, args ...ArgOption) (err error) {
	logger := log.GetCurrentLogger(ctx)
	if !op.isReady {
		logger.Error("rabbitmq connection is not ready, push cancel")
		return errors.New("connection is not ready")
	}
	channel, err := op.getChannel(ctx, false, msg.exchangeName, msg.queueName)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	msg.Validate()

	table := amqp.Table{}
	for _, arg := range args {
		arg(table)
	}
	if msg.exchangeName != "" {
		// 用于检查交换机是否存在,已经存在不需要重复声明
		err = channel.ExchangeDeclarePassive(msg.exchangeName, string(msg.exchangeType), true, false, false, true, nil)
		if err != nil {
			// 注册交换机
			// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
			// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
			err = channel.ExchangeDeclare(msg.exchangeName, string(msg.exchangeType), true, false, false, true, nil)
			if err != nil {
				logger.Error(fmt.Sprintf("MQ failed to declare the exchange:%s \n", err))
				return
			}
		}

		// 交换机绑定队列处理
		for queue, bindingKey := range msg.bindingKeyMap {
			_, err = channel.QueueDeclarePassive(queue, true, false, false, true, table)
			if err != nil {
				// 队列不存在,声明队列
				// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
				// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
				_, err = channel.QueueDeclare(queue, true, false, false, true, table)
				if err != nil {
					logger.Error(fmt.Sprintf("MQ declare queue failed:%s \n", err))
					return
				}
			}

			// 队列绑定
			err = channel.QueueBind(queue, bindingKey, msg.exchangeName, true, nil)
			if err != nil {
				logger.Error(fmt.Sprintf("MQ binding queue failed:%s \n", err))
				return
			}
		}
	}

	if msg.queueName != "" {
		// 用于检查队列是否存在,已经存在不需要重复声明
		_, err = channel.QueueDeclarePassive(msg.queueName, true, false, false, true, table)
		if err != nil {
			// 队列不存在,声明队列
			// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
			// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
			_, err = channel.QueueDeclare(msg.queueName, true, false, false, true, table)
			if err != nil {
				logger.Error(fmt.Sprintf("MQ declare queue failed:%s \n", err))
				return
			}
		}
	}

	publishingMsg := amqp.Publishing{
		ContentType: "text/plain",
		Timestamp:   times.Now(),
		Body:        msg.content,
	}
	if msg.priority > 0 {
		publishingMsg.Priority = msg.priority
	}
	// 发送任务消息
	err = channel.PublishWithContext(ctx, msg.exchangeName, msg.routingKey, false, false, publishingMsg)
	if err != nil {
		logger.Error(fmt.Sprintf("MQ task failed to be sent:%s \n", err))
		return
	}
	return
}

type ConsumeParam struct {
	exchangeName string
	routingKey   string
	queueName    string
	fetchCount   int
}

func (op *RabbitOperator) ConsumeMessage(ctx context.Context, param *ConsumeParam, args ...ArgOption) (msgs <-chan amqp.Delivery, err error) {
	logger := log.GetCurrentLogger(ctx)
	if !op.isReady {
		logger.Error("rabbitmq connection is not ready, push cancel")
		err = errors.New("connection is not ready")
		return
	}
	channel, err := op.getChannel(ctx, true, param.exchangeName, param.queueName)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	if param.fetchCount == 0 {
		param.fetchCount = 1
	}

	table := amqp.Table{}
	for _, arg := range args {
		arg(table)
	}
	// 用于检查队列是否存在,已经存在不需要重复声明
	_, err = channel.QueueDeclarePassive(param.queueName, true, false, false, true, table)
	if err != nil {
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = channel.QueueDeclare(param.queueName, true, false, false, true, table)
		if err != nil {
			logger.Error(fmt.Sprintf("MQ declare queue failed:%s \n", err))
			return
		}
	}
	if param.exchangeName != "" {
		// 绑定任务
		err = channel.QueueBind(param.queueName, param.routingKey, param.exchangeName, true, nil)
		if err != nil {
			logger.Error(fmt.Sprintf("binding queue failed:%s \n", err))
			return
		}
	}

	// 获取消费通道,确保rabbitMQ一个一个发送消息
	err = channel.Qos(param.fetchCount, 0, true)
	if err != nil {
		logger.Error(fmt.Sprintf("open qos error:%s \n", err))
		return
	}
	msgs, err = channel.Consume(param.queueName, "", false, false, false, false, nil)
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
	if ok {
		err = value.(*amqp.Channel).Close()
		if err != nil {
			return
		}
		op.client.exchangeCh.Delete(exchangeName)
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

func (op *RabbitOperator) Close(ctx context.Context) (err error) {
	op.mu.Lock()
	defer op.mu.Unlock()
	logger := log.GetCurrentLogger(ctx)
	if atomic.LoadInt32(&op.closed) == Closed {
		return
	}

	err = op.client.conn.Close()
	if err != nil {
		logger.Error(fmt.Sprintf("close rabbitmq connection error: %v", err))
		return err
	}

	atomic.StoreInt32(&op.closed, Closed)
	close(op.closeCh)

	op.isReady = false
	return
}
