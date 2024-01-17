package rabbitmqx

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PushDelayBody 生产延迟消息body参数设置，兼容交换机和队列两种模式
type PushDelayBody struct {
	OpenConfirm   bool
	ConfirmNoWait bool
	ExchangeName  string
	ExchangeType  ExchangeType
	ExchangeArgs  amqp.Table
	RoutingKey    string
	QueueName     string
	Args          amqp.Table
	DelayTime     time.Duration
	amqp.Publishing
}

func (p *PushDelayBody) SetExchangeName(exchangeName string) *PushDelayBody {
	p.ExchangeName = exchangeName
	return p
}

func (p *PushDelayBody) SetExchangeType(exchangeType ExchangeType) *PushDelayBody {
	p.ExchangeType = exchangeType
	return p
}

func (p *PushDelayBody) SetRoutingKey(routingKey string) *PushDelayBody {
	p.RoutingKey = routingKey
	return p
}

func (p *PushDelayBody) SetQueueName(queueName string) *PushDelayBody {
	p.QueueName = queueName
	return p
}

func (p *PushDelayBody) SetConfirmMode(confirm, noWait bool) *PushDelayBody {
	p.OpenConfirm = confirm
	p.ConfirmNoWait = noWait
	return p
}

func (p *PushDelayBody) SetDeliveryMode(deliveryMode uint8) *PushDelayBody {
	p.DeliveryMode = deliveryMode
	return p
}

func (p *PushDelayBody) SetExpireTime(expire time.Duration) *PushDelayBody {
	p.DelayTime = expire
	return p
}

func (p *PushDelayBody) SetMsg(msg []byte) *PushDelayBody {
	p.Body = msg
	return p
}

func (p *PushDelayBody) SetQueueMaxPriority(priority uint8) *PushDelayBody {
	if p.Args == nil {
		p.Args = amqp.Table{}
	}
	p.Args["x-max-priority"] = priority
	return p
}

// PushBody 生产消息body参数设置，兼容交换机和队列两种模式
type PushBody struct {
	OpenConfirm   bool
	ConfirmNoWait bool

	// 交换机模式
	ExchangeName  string
	ExchangeType  ExchangeType
	ExchangeArgs  amqp.Table
	BindingKeyMap map[string]string
	RoutingKey    string
	QueueArgs     map[string]amqp.Table

	// 队列模式
	QueueName string
	Args      amqp.Table

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
}

func (p *PushBody) SetPriority(priority uint8) *PushBody {
	p.Priority = priority
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
	if p.BindingKeyMap == nil {
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
	p.OpenConfirm = confirm
	p.ConfirmNoWait = noWait
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

func (p *PushBody) setArgs(key string, value any, queues ...string) *PushBody {
	if p.QueueArgs == nil {
		p.QueueArgs = make(map[string]amqp.Table)
	}
	if p.Args == nil {
		p.Args = amqp.Table{}
	}
	for _, queue := range queues {
		if queue == p.QueueName {
			p.Args[key] = value
		}
		table, ok := p.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table[key] = value
		p.QueueArgs[queue] = table
	}
	return p
}

// SetXMaxPriority 设置该队列中的消息的优先级最大值.发布消息的时候,可以指定消息的优先级,优先级高的先被消费.
// 如果没有设置该参数,那么该队列不支持消息优先级功能. 也就是说,就算发布消息的时候传入了优先级的值,也不会起什么作用.可设置0-255,一般设置为10即可
func (p *PushBody) SetXMaxPriority(priority int, queues ...string) *PushBody {
	return p.setArgs("x-max-priority", priority, queues...)
}

// SetXMaxLength 队列存放最大就绪消息数量，超过限制时，从头部丢弃消息
//
//	默认最大数量限制与操作系统有关。
func (p *PushBody) SetXMaxLength(length int, queues ...string) *PushBody {
	return p.setArgs("x-max-length", length, queues...)
}

// SetXMaxLengthBytes 队列存放的所有消息总大小，超过限制时，从头部丢弃消息
func (p *PushBody) SetXMaxLengthBytes(size int, queues ...string) *PushBody {
	return p.setArgs("x-max-length-bytes", size, queues...)
}

// SetOverFlowMode
// 队列溢出行为，这将决定当队列达到设置的最大长度或者最大的存储空间时发送到消息队列的消息的处理方式；
// 有效的值是：
// 　　drop-head（删除queue头部的消息）、
// 　　reject-publish（最近发来的消息将被丢弃）、
// 　　reject-publish-dlx（拒绝发送消息到死信交换器）
// 类型为quorum 的queue只支持drop-head;
func (p *PushBody) SetOverFlowMode(mode string, queues ...string) *PushBody {
	return p.setArgs("x-overflow", mode, queues...)
}

// SetDeadLetterRoutingKey 设置死信交换机的路由键名称。如果未设置，将使用消息的原始路由密钥。
func (p *PushBody) SetDeadLetterRoutingKey(routingKey string, queues ...string) *PushBody {
	return p.setArgs("x-dead-letter-routing-key", routingKey, queues...)
}

// SetDeadLetterExchange 设置死信交换机的名称
func (p *PushBody) SetDeadLetterExchange(exchange string, queues ...string) *PushBody {
	return p.setArgs("x-dead-letter-exchange", exchange, queues...)
}

// SetMsgTTL 发送到队列的消息可以存活多长时间（毫秒）。简单来说,就是队列中消息的过期时间
func (p *PushBody) SetMsgTTL(duration time.Duration, queues ...string) *PushBody {
	return p.setArgs("x-message-ttl", duration, queues...)
}

// SetQueueMode {"x-queue-mode","lazy" }设置队列为懒人模式.
// 该模式下的队列会先将交换机推送过来的消息(尽可能多的)保存在磁盘上,以减少内存的占用.
// 当消费者开始消费的时候才加载到内存中;如果没有设置懒人模式,队列则会直接利用内存缓存,以最快的速度传递消息.
func (p *PushBody) SetQueueMode(mode string, queues ...string) *PushBody {
	return p.setArgs("x-queue-mode", mode, queues...)
}

// SetExpireTime 队列在被自动删除（毫秒）之前可以存活多长时间。简单来说,就是队列的过期时间
func (p *PushBody) SetExpireTime(expire time.Duration, queues ...string) *PushBody {
	return p.setArgs("x-expires", expire, queues...)
}

// SetQueueMasterLocator 将队列设置为主位置模式，确定在节点集群上声明时队列主机所在的规则。
//
//	min-masters - 选择承载最小绑定主机数量的节点
//	client-local - 选择客户机声明队列连接到的节点
//	random  - 随机选择一个节点
func (p *PushBody) SetQueueMasterLocator(mode string, queues ...string) *PushBody {
	return p.setArgs("x-queue-master-locator", mode, queues...)
}

// ExchangePushBody 交换机模式推送消息
type ExchangePushBody struct {
	OpenConfirm   bool
	ConfirmNoWait bool
	ExchangeName  string
	ExchangeType  ExchangeType
	ExchangeArgs  amqp.Table
	RoutingKey    string
	BindingKeyMap map[string]string
	QueueArgs     map[string]amqp.Table
	XMaxPriority  uint8
	amqp.Publishing
}

func (e *ExchangePushBody) Validate() {
	if string(e.ExchangeType) == "" {
		//  default exchangeType is fanout
		e.ExchangeType = ExchangeTypeFanout
	}

	for queue, bindKey := range e.BindingKeyMap {
		if bindKey == "" {
			e.BindingKeyMap[queue] = queue
		}
	}
}

func (e *ExchangePushBody) SetPriority(priority uint8) *ExchangePushBody {
	e.Priority = priority
	return e
}

func (e *ExchangePushBody) setArgs(key string, value any, queues ...string) *ExchangePushBody {
	if e.QueueArgs == nil {
		e.QueueArgs = make(map[string]amqp.Table)
	}
	for _, queue := range queues {
		table, ok := e.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table[key] = value
		e.QueueArgs[queue] = table
	}
	return e
}

// SetXMaxPriority 设置该队列中的消息的优先级最大值.发布消息的时候,可以指定消息的优先级,优先级高的先被消费.
// 如果没有设置该参数,那么该队列不支持消息优先级功能. 也就是说,就算发布消息的时候传入了优先级的值,也不会起什么作用.可设置0-255,一般设置为10即可
func (e *ExchangePushBody) SetXMaxPriority(priority int, queues ...string) *ExchangePushBody {
	return e.setArgs("x-max-priority", priority, queues...)
}

// SetXMaxLength 队列存放最大就绪消息数量，超过限制时，从头部丢弃消息
//
//	默认最大数量限制与操作系统有关。
func (e *ExchangePushBody) SetXMaxLength(length int, queues ...string) *ExchangePushBody {
	return e.setArgs("x-max-length", length, queues...)
}

// SetXMaxLengthBytes 队列存放的所有消息总大小，超过限制时，从头部丢弃消息
func (e *ExchangePushBody) SetXMaxLengthBytes(size int, queues ...string) *ExchangePushBody {
	return e.setArgs("x-max-length-bytes", size, queues...)
}

// SetOverFlowMode
// 队列溢出行为，这将决定当队列达到设置的最大长度或者最大的存储空间时发送到消息队列的消息的处理方式；
// 有效的值是：
// 　　drop-head（删除queue头部的消息）、
// 　　reject-publish（最近发来的消息将被丢弃）、
// 　　reject-publish-dlx（拒绝发送消息到死信交换器）
// 类型为quorum 的queue只支持drop-head;
func (e *ExchangePushBody) SetOverFlowMode(mode string, queues ...string) *ExchangePushBody {
	return e.setArgs("x-overflow", mode, queues...)
}

// SetDeadLetterRoutingKey 设置死信交换机的路由键名称。如果未设置，将使用消息的原始路由密钥。
func (e *ExchangePushBody) SetDeadLetterRoutingKey(routingKey string, queues ...string) *ExchangePushBody {
	return e.setArgs("x-dead-letter-routing-key", routingKey, queues...)
}

// SetDeadLetterExchange 设置死信交换机的名称
func (e *ExchangePushBody) SetDeadLetterExchange(exchange string, queues ...string) *ExchangePushBody {
	return e.setArgs("x-dead-letter-exchange", exchange, queues...)
}

// SetMsgTTL 发送到队列的消息可以存活多长时间（毫秒）。简单来说,就是队列中消息的过期时间
func (e *ExchangePushBody) SetMsgTTL(duration time.Duration, queues ...string) *ExchangePushBody {
	return e.setArgs("x-message-ttl", duration, queues...)
}

// SetQueueMode {"x-queue-mode","lazy" }设置队列为懒人模式.
// 该模式下的队列会先将交换机推送过来的消息(尽可能多的)保存在磁盘上,以减少内存的占用.
// 当消费者开始消费的时候才加载到内存中;如果没有设置懒人模式,队列则会直接利用内存缓存,以最快的速度传递消息.
func (e *ExchangePushBody) SetQueueMode(mode string, queues ...string) *ExchangePushBody {
	return e.setArgs("x-queue-mode", mode, queues...)
}

// SetExpireTime 队列在被自动删除（毫秒）之前可以存活多长时间。简单来说,就是队列的过期时间
func (e *ExchangePushBody) SetExpireTime(expire time.Duration, queues ...string) *ExchangePushBody {
	return e.setArgs("x-expires", expire, queues...)
}

// SetQueueMasterLocator 将队列设置为主位置模式，确定在节点集群上声明时队列主机所在的规则。
//
//	min-masters - 选择承载最小绑定主机数量的节点
//	client-local - 选择客户机声明队列连接到的节点
//	random  - 随机选择一个节点
func (e *ExchangePushBody) SetQueueMasterLocator(mode string, queues ...string) *ExchangePushBody {
	return e.setArgs("x-queue-master-locator", mode, queues...)
}

func (e *ExchangePushBody) SetExchangeName(exchangeName string) *ExchangePushBody {
	e.ExchangeName = exchangeName
	return e
}

func (e *ExchangePushBody) SetExchangeType(exchangeType ExchangeType) *ExchangePushBody {
	e.ExchangeType = exchangeType
	return e
}

func (e *ExchangePushBody) BindQueue(queueName, bindingKey string) *ExchangePushBody {
	if e.BindingKeyMap == nil {
		e.BindingKeyMap = map[string]string{}
	}
	if bindingKey == "" {
		bindingKey = queueName
	}
	e.BindingKeyMap[queueName] = bindingKey
	return e
}

func (e *ExchangePushBody) SetRoutingKey(routingKey string) *ExchangePushBody {
	e.RoutingKey = routingKey
	return e
}

func (e *ExchangePushBody) SetConfirmMode(open, noWait bool) *ExchangePushBody {
	e.OpenConfirm = open
	e.ConfirmNoWait = noWait
	return e
}

func (e *ExchangePushBody) SetDeliveryMode(deliveryMode uint8) *ExchangePushBody {
	e.DeliveryMode = deliveryMode
	return e
}

func (e *ExchangePushBody) SetMsg(msg []byte) *ExchangePushBody {
	e.Body = msg
	return e
}

type QueuePushBody struct {
	OpenConfirm   bool
	ConfirmNoWait bool
	QueueName     string
	Args          amqp.Table
	amqp.Publishing
}

func (q *QueuePushBody) Validate() {
}

func (q *QueuePushBody) SetPriority(priority uint8) *QueuePushBody {
	q.Priority = priority
	return q
}

func (q *QueuePushBody) setArgs(key string, value any) *QueuePushBody {
	if q.Args == nil {
		q.Args = amqp.Table{}
	}
	q.Args[key] = value
	return q
}

// SetXMaxPriority 设置该队列中的消息的优先级最大值.发布消息的时候,可以指定消息的优先级,优先级高的先被消费.
// 如果没有设置该参数,那么该队列不支持消息优先级功能. 也就是说,就算发布消息的时候传入了优先级的值,也不会起什么作用.可设置0-255,一般设置为10即可
func (q *QueuePushBody) SetXMaxPriority(priority int) *QueuePushBody {
	return q.setArgs("x-max-priority", priority)
}

// SetXMaxLength 队列存放最大就绪消息数量，超过限制时，从头部丢弃消息
//
//	默认最大数量限制与操作系统有关。
func (q *QueuePushBody) SetXMaxLength(length int) *QueuePushBody {
	return q.setArgs("x-max-length", length)
}

// SetXMaxLengthBytes 队列存放的所有消息总大小，超过限制时，从头部丢弃消息
func (q *QueuePushBody) SetXMaxLengthBytes(size int) *QueuePushBody {
	return q.setArgs("x-max-length-bytes", size)
}

// SetOverFlowMode
// 队列溢出行为，这将决定当队列达到设置的最大长度或者最大的存储空间时发送到消息队列的消息的处理方式；
// 有效的值是：
// 　　drop-head（删除queue头部的消息）、
// 　　reject-publish（最近发来的消息将被丢弃）、
// 　　reject-publish-dlx（拒绝发送消息到死信交换器）
// 类型为quorum 的queue只支持drop-head;
func (q *QueuePushBody) SetOverFlowMode(mode string) *QueuePushBody {
	return q.setArgs("x-overflow", mode)
}

// SetDeadLetterRoutingKey 设置死信交换机的路由键名称。如果未设置，将使用消息的原始路由密钥。
func (q *QueuePushBody) SetDeadLetterRoutingKey(routingKey string) *QueuePushBody {
	return q.setArgs("x-dead-letter-routing-key", routingKey)
}

// SetDeadLetterExchange 设置死信交换机的名称
func (q *QueuePushBody) SetDeadLetterExchange(exchange string) *QueuePushBody {
	return q.setArgs("x-dead-letter-exchange", exchange)
}

// SetMsgTTL 发送到队列的消息可以存活多长时间（毫秒）。简单来说,就是队列中消息的过期时间
func (q *QueuePushBody) SetMsgTTL(duration time.Duration) *QueuePushBody {
	return q.setArgs("x-message-ttl", duration)
}

// SetQueueMode {"x-queue-mode","lazy" }设置队列为懒人模式.
// 该模式下的队列会先将交换机推送过来的消息(尽可能多的)保存在磁盘上,以减少内存的占用.
// 当消费者开始消费的时候才加载到内存中;如果没有设置懒人模式,队列则会直接利用内存缓存,以最快的速度传递消息.
func (q *QueuePushBody) SetQueueMode(mode string) *QueuePushBody {
	return q.setArgs("x-queue-mode", mode)
}

// SetExpireTime 队列在被自动删除（毫秒）之前可以存活多长时间。简单来说,就是队列的过期时间
func (q *QueuePushBody) SetExpireTime(expire time.Duration) *QueuePushBody {
	return q.setArgs("x-expires", expire)
}

// SetQueueMasterLocator 将队列设置为主位置模式，确定在节点集群上声明时队列主机所在的规则。
//
//	min-masters - 选择承载最小绑定主机数量的节点
//	client-local - 选择客户机声明队列连接到的节点
//	random  - 随机选择一个节点
func (q *QueuePushBody) SetQueueMasterLocator(mode string) *QueuePushBody {
	return q.setArgs("x-queue-master-locator", mode)
}

func (q *QueuePushBody) SetQueueName(queueName string) *QueuePushBody {
	q.QueueName = queueName
	return q
}

func (q *QueuePushBody) SetConfirmMode(open, noWait bool) *QueuePushBody {
	q.OpenConfirm = open
	q.ConfirmNoWait = noWait
	return q
}

func (q *QueuePushBody) SetDeliveryMode(deliveryMode uint8) *QueuePushBody {
	q.DeliveryMode = deliveryMode
	return q
}

func (q *QueuePushBody) SetMsg(msg []byte) *QueuePushBody {
	q.Body = msg
	return q
}

// ConsumeBody 消费body参数设置
type ConsumeBody struct {
	ExchangeName string
	RoutingKey   string
	QueueName    string
	AutoAck      bool
	QueueArgs    amqp.Table
	FetchCount   int
	XPriority    uint8
}

func (c *ConsumeBody) SetXPriority(xPriority uint8) *ConsumeBody {
	c.XPriority = xPriority
	return c
}

func (c *ConsumeBody) setArgs(key string, value any) *ConsumeBody {
	if c.QueueArgs == nil {
		c.QueueArgs = amqp.Table{}
	}
	c.QueueArgs[key] = value
	return c
}

// SetXMaxPriority 设置该队列中的消息的优先级最大值.发布消息的时候,可以指定消息的优先级,优先级高的先被消费.
// 如果没有设置该参数,那么该队列不支持消息优先级功能. 也就是说,就算发布消息的时候传入了优先级的值,也不会起什么作用.可设置0-255,一般设置为10即可
func (c *ConsumeBody) SetXMaxPriority(priority int) *ConsumeBody {
	return c.setArgs("x-max-priority", priority)
}

// SetXMaxLength 队列存放最大就绪消息数量，超过限制时，从头部丢弃消息
//
//	默认最大数量限制与操作系统有关。
func (c *ConsumeBody) SetXMaxLength(length int) *ConsumeBody {
	return c.setArgs("x-max-length", length)
}

// SetXMaxLengthBytes 队列存放的所有消息总大小，超过限制时，从头部丢弃消息
func (c *ConsumeBody) SetXMaxLengthBytes(size int) *ConsumeBody {
	return c.setArgs("x-max-length-bytes", size)
}

// SetOverFlowMode
// 队列溢出行为，这将决定当队列达到设置的最大长度或者最大的存储空间时发送到消息队列的消息的处理方式；
// 有效的值是：
// 　　drop-head（删除queue头部的消息）、
// 　　reject-publish（最近发来的消息将被丢弃）、
// 　　reject-publish-dlx（拒绝发送消息到死信交换器）
// 类型为quorum 的queue只支持drop-head;
func (c *ConsumeBody) SetOverFlowMode(mode string) *ConsumeBody {
	return c.setArgs("x-overflow", mode)
}

// SetDeadLetterRoutingKey 设置死信交换机的路由键名称。如果未设置，将使用消息的原始路由密钥。
func (c *ConsumeBody) SetDeadLetterRoutingKey(routingKey string) *ConsumeBody {
	return c.setArgs("x-dead-letter-routing-key", routingKey)
}

// SetDeadLetterExchange 设置死信交换机的名称
func (c *ConsumeBody) SetDeadLetterExchange(exchange string) *ConsumeBody {
	return c.setArgs("x-dead-letter-exchange", exchange)
}

// SetMsgTTL 发送到队列的消息可以存活多长时间（毫秒）。简单来说,就是队列中消息的过期时间
func (c *ConsumeBody) SetMsgTTL(duration time.Duration) *ConsumeBody {
	return c.setArgs("x-message-ttl", duration)
}

// SetQueueMode {"x-queue-mode","lazy" }设置队列为懒人模式.
// 该模式下的队列会先将交换机推送过来的消息(尽可能多的)保存在磁盘上,以减少内存的占用.
// 当消费者开始消费的时候才加载到内存中;如果没有设置懒人模式,队列则会直接利用内存缓存,以最快的速度传递消息.
func (c *ConsumeBody) SetQueueMode(mode string) *ConsumeBody {
	return c.setArgs("x-queue-mode", mode)
}

// SetExpireTime 队列在被自动删除（毫秒）之前可以存活多长时间。简单来说,就是队列的过期时间
func (c *ConsumeBody) SetExpireTime(expire time.Duration) *ConsumeBody {
	return c.setArgs("x-expires", expire)
}

// SetQueueMasterLocator 将队列设置为主位置模式，确定在节点集群上声明时队列主机所在的规则。
//
//	min-masters - 选择承载最小绑定主机数量的节点
//	client-local - 选择客户机声明队列连接到的节点
//	random  - 随机选择一个节点
func (c *ConsumeBody) SetQueueMasterLocator(mode string) *ConsumeBody {
	return c.setArgs("x-queue-master-locator", mode)
}

func (c *ConsumeBody) SetExchangeName(exchangeName string) *ConsumeBody {
	c.ExchangeName = exchangeName
	return c
}

func (c *ConsumeBody) SetRoutingKey(routingKey string) *ConsumeBody {
	c.RoutingKey = routingKey
	return c
}

func (c *ConsumeBody) SetQueueName(queueName string) *ConsumeBody {
	c.QueueName = queueName
	return c
}

func (c *ConsumeBody) SetAckMode(autoAck bool) *ConsumeBody {
	c.AutoAck = autoAck
	return c
}
