# goredis

Redis 客户端封装，自动检测并支持 Single/Cluster/Sentinel 三种部署模式，内置延迟消息队列和发布订阅功能。

## 目录结构

```
goredis/
├── redis.go      # 主操作接口（自动模式检测）
├── queue.go      # 延迟队列实现
├── single/       # 单节点模式
├── cluster/      # 集群模式
└── sentinel/     # 哨兵模式
```

## 模式自动检测

- 配置了 `MasterName` → Sentinel 模式
- 配置了多个 `Addrs` → Cluster 模式
- 其他 → Single 模式

## 使用示例

### 基本操作

```go
import "github.com/jasonlabz/potato/goredis"

op, err := goredis.NewRedisOperator(&goredis.Config{
    Addrs:    []string{"127.0.0.1:6379"},
    Password: "xxx",
    IndexDB:  0,
})

// String 操作
op.Set(ctx, "key", "value")
val, _ := op.Get(ctx, "key")

// Hash 操作
op.HSet(ctx, "hash", "field", "value")
val, _ = op.HGet(ctx, "hash", "field")

// List 操作
op.LPush(ctx, "list", "a", "b", "c")
val, _ = op.LPop(ctx, "list")

// Set 操作
op.SAdd(ctx, "set", "member1", "member2")
members, _ := op.SMembers(ctx, "set")

// Sorted Set 操作
op.ZAdd(ctx, "zset", redis.Z{Score: 1.0, Member: "a"})
```

### 延迟队列

```go
// 推送延迟消息（10 秒后可消费）
op.PushDelayMessage(ctx, "topic", "payload", 10*time.Second)

// 推送即时消息
op.PushMessage(ctx, "topic", "payload")

// 消费消息
msg, _ := op.GetMessage(ctx, "topic")

// 发布订阅
op.Publish(ctx, "channel", "message")
op.Subscribe(ctx, handler, "channel")
```

### 配置集成

```yaml
redis:
  endpoints:
    - "127.0.0.1:6379"
  password: "xxx"
  index_db: 0
  min_idle_conns: 10
  max_idle_conns: 50
  max_active_conns: 100
  max_retry_times: 5
  master_name: ""         # 非空则为 Sentinel 模式
  sentinel_username: ""
  sentinel_password: ""
```
