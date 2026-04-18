# rabbitmqx

RabbitMQ 客户端封装，支持五种队列模型、延迟队列、优先级队列和断线重连。

基于一个 Connection 对应多个 Channel 的使用理念封装，一个队列/交换机对应一个 Channel。

## 核心类型

- `RabbitMQOperator` - 主操作实例
- `MQConfig` - 连接配置
- `PushBody` / `PushDelayBody` / `ExchangePushBody` / `QueuePushBody` - 消息体（Fluent API）
- `ConsumeBody` - 消费者配置

## 配置文件

默认读取 `conf/application.yaml`：

```yaml
rabbitmq:
  enable: false                # 是否启用
  strict: true                 # 是否为下游必需，如为 true 则启动时 panic
  host: "127.0.0.1"
  port: 5672
  username: guest
  password: guest
  limit_conf:
    attempt_times: 3           # 重试次数
    retry_wait_time: 3000      # 重试等待时间（ms）
    prefetch_count: 100        # 队列预读取数量
    timeout: 5000              # 超时时间（ms）
    queue_limit: 0             # 队列长度限制
```

## 使用示例

### 初始化

```go
import "github.com/jasonlabz/potato/rabbitmqx"

// 方式一：自动读取配置初始化（导入包即可）
// 方式二：手动初始化
operator, err := rabbitmqx.NewRabbitMQOperator(&rabbitmqx.MQConfig{
    Username: "guest",
    Password: "guest",
    Host:     "127.0.0.1",
    Port:     5672,
})
defer operator.Close()
```

### 生产消息

```go
// 通过交换机推送
msg := &rabbitmqx.PushBody{}
err = operator.Push(ctx, msg.SetMsg([]byte("hello")).
    SetExchangeName("exchange").
    SetPriority(1).
    BindQueue("queue_name", "routing_key").
    SetXMaxPriority(4, "queue_name"))

// 交换机推送（绑定多个队列）
emsg := &rabbitmqx.ExchangePushBody{}
err = operator.PushExchange(ctx, emsg.SetMsg([]byte("hello")).
    SetExchangeName("exchange").
    BindQueue("queue1", "key1").
    BindQueue("queue2", "key2"))

// 直接推送到队列
qmsg := &rabbitmqx.QueuePushBody{}
err = operator.PushQueue(ctx, qmsg.SetMsg([]byte("hello")).
    SetQueueName("queue").
    SetPriority(1).
    SetXMaxPriority(4))

// 延迟消息
dmsg := &rabbitmqx.PushDelayBody{}
err = operator.PushDelayMessage(ctx, dmsg.SetMsg([]byte("hello")).
    SetExchangeName("delay_exchange").
    BindQueue("delay_queue", "delay_key").
    SetDelayTime(25*time.Second))
```

### 消费消息

```go
body := &rabbitmqx.ConsumeBody{}
deliveries, err := operator.Consume(ctx, body.SetQueueName("queue").SetPrefetchCount(10))
for delivery := range deliveries {
    fmt.Println("get msg: " + delivery.MessageId + " -- " + string(delivery.Body))
    operator.Ack(ctx, delivery)
}
```

## 特性

- 五种队列模型支持（Simple/Work/Publish/Subscribe/Routing/Topic）
- 延迟队列、优先级队列
- 断线自动重连
- 持续消费
- Confirm 模式
