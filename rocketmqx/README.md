# rocketmqx

RocketMQ 生产者/消费者封装，基于 [apache/rocketmq-client-go](https://github.com/apache/rocketmq-client-go)，支持集群/广播消费和延迟消息。

## 核心类型

- `RocketMQOperator` - 主操作实例
- `MQConfig` - 连接配置
- `ProduceMessage` - 生产消息体，支持 Tags、Keys、DelayLevel
- `ConsumeConfig` - 消费者配置

## 使用示例

### 初始化

```go
import "github.com/jasonlabz/potato/rocketmqx"

operator, err := rocketmqx.NewRocketMQOperator(&rocketmqx.MQConfig{
    NameServer: []string{"127.0.0.1:9876"},
    GroupName:  "my-group",
})
defer operator.Close(ctx)
```

### 生产消息

```go
msg := &rocketmqx.ProduceMessage{}
msg.SetTopic("my-topic").SetTags("tagA").SetKeys("key1").SetBody([]byte("hello"))
err := operator.Push(ctx, msg)

// 延迟消息
msg.SetDelayLevel(3) // 延迟 10 秒
err = operator.Push(ctx, msg)
```

### 消费消息

```go
config := &rocketmqx.ConsumeConfig{}
config.SetTopic("my-topic").SetGroupName("my-group")
err := operator.ConsumeWithHandler(ctx, config, func(msg *primitive.MessageExt) error {
    fmt.Println(string(msg.Body))
    return nil
})
```

### 配置集成

```yaml
rocketmq:
  name_server:
    - "127.0.0.1:9876"
  group_name: "my-group"
```

## 特性

- 集群消费 / 广播消费模式
- 延迟消息（18 个延迟级别）
- 消息过滤（Tags）
- 顺序消息
