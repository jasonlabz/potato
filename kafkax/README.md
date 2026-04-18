# kafkax

Kafka 生产者/消费者封装，基于 [segmentio/kafka-go](https://github.com/segmentio/kafka-go)，支持 SASL 认证、批量发送和 Topic 管理。

## 核心类型

- `KafkaOperator` - 主操作实例，管理 Writer/Reader 连接池
- `MQConfig` - Kafka 连接配置
- `ProduceMessage` - 生产消息体（Fluent API）
- `ConsumeConfig` - 消费者配置

## SASL 认证

支持以下认证机制：
- PLAIN
- SCRAM-SHA-256
- SCRAM-SHA-512

## 使用示例

### 初始化

```go
import "github.com/jasonlabz/potato/kafkax"

operator, err := kafkax.NewKafkaOperator(&kafkax.MQConfig{
    BootstrapServers: "localhost:9092",
    GroupID:          "my-group",
    SecurityProtocol: "PLAINTEXT",
})
defer operator.Close(ctx)
```

### 生产消息

```go
// 单条发送
msg := &kafkax.ProduceMessage{}
msg.SetTopic("my-topic").SetKey([]byte("key")).SetValue([]byte("value"))
err := operator.Push(ctx, msg)

// 批量发送（内置重试机制）
msgs := []*kafkax.ProduceMessage{msg1, msg2, msg3}
err = operator.PushBatch(ctx, "my-topic", msgs)
```

### 消费消息

```go
// Channel 方式
config := &kafkax.ConsumeConfig{}
config.SetTopic("my-topic").SetGroupID("my-group")
ch, err := operator.Consume(ctx, config)
for msg := range ch {
    fmt.Println(string(msg.Value))
}

// Handler 方式
err = operator.ConsumeWithHandler(ctx, config, func(msg kafka.Message) error {
    fmt.Println(string(msg.Value))
    return nil
})
```

### Topic 管理

```go
operator.CreateTopic(ctx, "new-topic", 3, 1)  // 3 分区，1 副本
topics, _ := operator.ListTopics(ctx)
operator.DeleteTopic(ctx, "old-topic")
```

### 配置集成

```yaml
kafka:
  topic: ["my-topic"]
  group_id: "my-group"
  bootstrap_servers: "host1:9092,host2:9092"
  security_protocol: "PLAINTEXT"
  sasl_mechanism: "PLAIN"
  sasl_username: "user"
  sasl_password: "pass"
```
