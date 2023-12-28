package rabbitmqx

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Producer 定义生产者接口
type Producer interface {
	PushMsgContent() string
}

// Receiver 定义接收者接口
type Receiver interface {
	Consumer([]byte) error
}

// RabbitMQ 定义RabbitMQ对象
type RabbitMQ struct {
	connection   *amqp.Connection
	channel      *amqp.Channel
	config       MQConfig   // 队列连接信息
	ModeType     string     // 模式类型
	queueName    string     // 队列名称
	routingKey   string     // key名称
	exchangeName string     // 交换机名称
	exchangeType string     // 交换机类型
	producerList []Producer // 该队列的生产者
	receiverList []Receiver // 该队列的消费者
}

// 链接rabbitMQ
func (r *RabbitMQ) mqConnect() {
	var err error
	RabbitUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", "guest", "guest", "******", 5673)
	mqConn, err := amqp.Dial(RabbitUrl)
	r.connection = mqConn // 赋值给RabbitMQ对象
	if err != nil {
		fmt.Printf("MQ打开链接失败:%s \n", err)
	}
	mqChan, err := mqConn.Channel()
	r.channel = mqChan // 赋值给RabbitMQ对象
	if err != nil {
		fmt.Printf("MQ打开管道失败:%s \n", err)
	}
}

// 关闭RabbitMQ连接
func (r *RabbitMQ) mqClose() {
	// 先关闭管道,再关闭链接
	err := r.channel.Close()
	if err != nil {
		fmt.Printf("MQ管道关闭失败:%s \n", err)
	}
	err = r.connection.Close()
	if err != nil {
		fmt.Printf("MQ链接关闭失败:%s \n", err)
	}
}

// New 创建一个新的操作对象
func New(q *QueueExchange) *RabbitMQ {
	return &RabbitMQ{
		queueName:    q.QueueName,
		routingKey:   q.RoutingKey,
		exchangeName: q.ExchangeName,
		exchangeType: q.ExchangeType,
	}
}

// 启动RabbitMQ客户端,并初始化
func (r *RabbitMQ) Start() {
	// 开启监听生产者发送任务
	for _, producer := range r.producerList {
		go r.listenProducer(producer)
	}
	// 开启监听接收者接收任务
	for _, receiver := range r.receiverList {
		go r.listenReceiver(receiver)
	}
	time.Sleep(1 * time.Second)
}

// 注册发送指定队列指定路由的生产者
func (r *RabbitMQ) RegisterProducer(producer Producer) {
	r.producerList = append(r.producerList, producer)
}

// 发送任务
func (r *RabbitMQ) listenProducer(producer Producer) {
	// 验证链接是否正常,否则重新链接
	if r.channel == nil {
		r.mqConnect()
	}
	// 用于检查队列是否存在,已经存在不需要重复声明
	_, err := r.channel.QueueDeclarePassive(r.queueName, true, false, false, true, nil)
	if err != nil {
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = r.channel.QueueDeclare(r.queueName, true, false, false, true, nil)
		if err != nil {
			fmt.Printf("MQ注册队列失败:%s \n", err)
			return
		}
	}
	// 队列绑定
	err = r.channel.QueueBind(r.queueName, r.routingKey, r.exchangeName, true, nil)
	if err != nil {
		fmt.Printf("MQ绑定队列失败:%s \n", err)
		return
	}
	// 用于检查交换机是否存在,已经存在不需要重复声明
	err = r.channel.ExchangeDeclarePassive(r.exchangeName, r.exchangeType, true, false, false, true, nil)
	if err != nil {
		// 注册交换机
		// name:交换机名称,kind:交换机类型,durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;
		// noWait:是否非阻塞, true为是,不等待RMQ返回信息;args:参数,传nil即可; internal:是否为内部
		err = r.channel.ExchangeDeclare(r.exchangeName, r.exchangeType, true, false, false, true, nil)
		if err != nil {
			fmt.Printf("MQ注册交换机失败:%s \n", err)
			return
		}
	}
	// 发送任务消息
	err = r.channel.Publish(r.exchangeName, r.routingKey, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(producer.PushMsgContent()),
	})
	if err != nil {
		fmt.Printf("MQ任务发送失败:%s \n", err)
		return
	}
}

// 注册接收指定队列指定路由的数据接收者
func (r *RabbitMQ) RegisterReceiver(receiver Receiver) {
	r.receiverList = append(r.receiverList, receiver)
}

// 监听接收者接收任务
func (r *RabbitMQ) listenReceiver(receiver Receiver) {
	// 处理结束关闭链接
	defer r.mqClose()
	// 验证链接是否正常
	if r.channel == nil {
		r.mqConnect()
	}
	// 用于检查队列是否存在,已经存在不需要重复声明
	_, err := r.channel.QueueDeclarePassive(r.queueName, true, false, false, true, nil)
	if err != nil {
		// 队列不存在,声明队列
		// name:队列名称;durable:是否持久化,队列存盘,true服务重启后信息不会丢失,影响性能;autoDelete:是否自动删除;noWait:是否非阻塞,
		// true为是,不等待RMQ返回信息;args:参数,传nil即可;exclusive:是否设置排他
		_, err = r.channel.QueueDeclare(r.queueName, true, false, false, true, nil)
		if err != nil {
			fmt.Printf("MQ注册队列失败:%s \n", err)
			return
		}
	}
	// 绑定任务
	err = r.channel.QueueBind(r.queueName, r.routingKey, r.exchangeName, true, nil)
	if err != nil {
		fmt.Printf("绑定队列失败:%s \n", err)
		return
	}
	// 获取消费通道,确保rabbitMQ一个一个发送消息
	err = r.channel.Qos(1, 0, true)
	msgList, err := r.channel.Consume(r.queueName, "", false, false, false, false, nil)
	if err != nil {
		fmt.Printf("获取消费通道异常:%s \n", err)
		return
	}
	for msg := range msgList {
		// 处理数据
		err := receiver.Consumer(msg.Body)
		if err != nil {
			err = msg.Ack(true)
			if err != nil {
				fmt.Printf("确认消息未完成异常:%s \n", err)
				return
			}
		} else {
			// 确认消息,必须为false
			err = msg.Ack(false)
			if err != nil {
				fmt.Printf("确认消息完成异常:%s \n", err)
				return
			}
			return
		}
	}
}
