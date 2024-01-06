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

func (p *PushBody) SetQueueMaxPriority(priority uint8, queues ...string) *PushBody {
	if p.QueueArgs == nil {
		p.QueueArgs = make(map[string]amqp.Table)
	}
	if p.Args == nil {
		p.Args = amqp.Table{}
	}
	for _, queue := range queues {
		if queue == p.QueueName {
			p.Args["x-max-priority"] = priority
		}
		table, ok := p.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table["x-max-priority"] = priority
		p.QueueArgs[queue] = table
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

func (p *PushBody) SetMsgTTL(duration time.Duration, queues ...string) *PushBody {
	if p.QueueArgs == nil {
		p.QueueArgs = make(map[string]amqp.Table)
	}
	if p.Args == nil {
		p.Args = amqp.Table{}
	}
	for _, queue := range queues {
		if queue == p.QueueName {
			p.Args["x-message-ttl"] = duration
		}
		table, ok := p.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table["x-message-ttl"] = duration
		p.QueueArgs[queue] = table
	}
	return p
}

func (p *PushBody) SetDeadLetterExchange(exchange, routingKey string, queues ...string) *PushBody {
	if p.QueueArgs == nil {
		p.QueueArgs = make(map[string]amqp.Table)
	}
	if p.Args == nil {
		p.Args = amqp.Table{}
	}
	for _, queue := range queues {
		if queue == p.QueueName {
			p.Args["x-dead-letter-exchange"] = exchange
			p.Args["x-dead-letter-routing-key"] = routingKey
		}
		table, ok := p.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table["x-dead-letter-exchange"] = exchange
		table["x-dead-letter-routing-key"] = routingKey
		p.QueueArgs[queue] = table
	}
	return p
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

func (e *ExchangePushBody) SetQueueMaxPriority(priority uint8, queues ...string) *ExchangePushBody {
	if e.QueueArgs == nil {
		e.QueueArgs = make(map[string]amqp.Table)
	}
	for _, queue := range queues {
		table, ok := e.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table["x-max-priority"] = priority
		e.QueueArgs[queue] = table
	}
	return e
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

func (e *ExchangePushBody) SetMsgTTL(duration time.Duration, queues ...string) *ExchangePushBody {
	if e.QueueArgs == nil {
		e.QueueArgs = make(map[string]amqp.Table)
	}
	for _, queue := range queues {
		table, ok := e.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table["x-message-ttl"] = duration
		e.QueueArgs[queue] = table
	}
	return e
}

func (e *ExchangePushBody) SetDeadLetterExchange(exchange, routingKey string, queues ...string) *ExchangePushBody {
	if e.QueueArgs == nil {
		e.QueueArgs = make(map[string]amqp.Table)
	}
	for _, queue := range queues {
		table, ok := e.QueueArgs[queue]
		if !ok {
			table = amqp.Table{}
		}
		table["x-dead-letter-exchange"] = exchange
		table["x-dead-letter-routing-key"] = routingKey
		e.QueueArgs[queue] = table
	}
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

func (q *QueuePushBody) SetQueueMaxPriority(priority uint8) *QueuePushBody {
	if q.Args == nil {
		q.Args = amqp.Table{}
	}
	q.Args["x-max-priority"] = priority
	return q
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

func (q *QueuePushBody) SetMsgTTL(duration time.Duration) *QueuePushBody {
	if q.Args == nil {
		q.Args = amqp.Table{}
	}
	q.Args["x-message-ttl"] = duration
	return q
}

func (q *QueuePushBody) SetDeadLetterExchange(exchange, routingKey string) *QueuePushBody {
	if q.Args == nil {
		q.Args = amqp.Table{}
	}
	q.Args["x-dead-letter-exchange"] = exchange
	q.Args["x-dead-letter-routing-key"] = routingKey
	return q
}

// ConsumeBody 消费body参数设置
type ConsumeBody struct {
	ExchangeName string
	RoutingKey   string
	QueueName    string
	QueueArgs    amqp.Table
	FetchCount   int
	XPriority    uint8
}

func (c *ConsumeBody) SetXPriority(xPriority uint8) *ConsumeBody {
	c.XPriority = xPriority
	return c
}

func (c *ConsumeBody) SetQueueMaxPriority(priority uint8) *ConsumeBody {
	if c.QueueArgs == nil {
		c.QueueArgs = amqp.Table{}
	}
	c.QueueArgs["x-max-priority"] = priority
	return c
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
