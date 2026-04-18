# Potato

Potato 是一个模块化的 Go 语言基础工具库，封装了后端开发中常用的基础组件，提供统一的 API 风格和开箱即用的能力。适用于快速搭建 Web 服务、数据处理管道、微服务等场景。

```
go get github.com/jasonlabz/potato
```

## 模块总览

| 分类 | 包名 | 说明 |
|------|------|------|
| **配置管理** | [configx](./configx/) | 多格式配置加载（YAML/JSON/TOML/INI），支持 ETCD 远程配置和热更新 |
| **常量定义** | [consts](./consts/) | 全局常量：上下文键名、HTTP Header、标点符号、API 版本号 |
| **日志** | [log](./log/) | 基于 Zap 的日志封装，支持上下文字段提取、文件轮转、分级输出 |
| **日志** | [slogx](./slogx/) | 基于 slog 的日志封装，提供与 log 包一致的上下文字段能力 |
| **Web 框架** | [ginx](./ginx/) | Gin 框架响应体标准化封装 |
| **Web 框架** | [ginmetrics](./ginmetrics/) | Gin 请求 Prometheus 指标采集中间件 |
| **Web 框架** | [middleware](./middleware/) | Gin 中间件集合：鉴权、上下文注入、请求日志、Panic 恢复 |
| **数据库** | [gormx](./gormx/) | GORM 封装，支持 MySQL/PostgreSQL/SQLServer/Oracle/SQLite/DM 多数据库及读写分离 |
| **缓存** | [gocache](./gocache/) | 基于 go-cache 的本地内存缓存 |
| **缓存** | [goredis](./goredis/) | Redis 客户端封装，支持 Single/Cluster/Sentinel 三种模式，内置延迟队列 |
| **消息队列** | [kafkax](./kafkax/) | Kafka 生产者/消费者封装，支持 SASL 认证、批量发送、Topic 管理 |
| **消息队列** | [rabbitmqx](./rabbitmqx/) | RabbitMQ 封装，支持五种队列模型、延迟队列、优先级队列、断线重连 |
| **消息队列** | [rocketmqx](./rocketmqx/) | RocketMQ 生产者/消费者封装，支持集群/广播消费、延迟消息 |
| **搜索引擎** | [es](./es/) | Elasticsearch v9 客户端封装，支持 CRUD、搜索、批量操作 |
| **MongoDB** | [mongo](./mongo/) | 泛型 MongoDB Repository 封装，支持 CRUD、聚合、分页、事务、Change Stream |
| **HTTP 客户端** | [httpx](./httpx/) | 基于 resty 的 HTTP 客户端封装 |
| **HTTP 客户端** | [ral](./ral/) | HTTP 请求抽象层，用于服务间调用 |
| **加解密** | [cryptox](./cryptox/) | 加解密套件：AES、DES、RSA、Base64、HMAC、MD5、SHA |
| **认证** | [jwtx](./jwtx/) | JWT Token 生成与解析 |
| **定时任务** | [cron](./cron/) | 基于 robfig/cron 的定时任务封装，支持秒级精度和 Fluent Builder |
| **并发池** | [poolx](./poolx/) | 基于 ants 的协程池封装 |
| **Kubernetes** | [kube](./kube/) | Kubernetes 客户端封装，支持 Deployment/Service/ConfigMap/PV/PVC 等资源操作 |
| **数据结构** | [bloom](./bloom/) | 布隆过滤器实现 |
| **数据结构** | [streams](./streams/) | 函数式流处理（Map/Filter/Reduce/Sort/Distinct 等） |
| **工具函数** | [utils](./utils/) | 通用工具函数集合（随机数、类型转换、指针操作等） |
| **工具函数** | [stringutil](./stringutil/) | 字符串处理工具 |
| **工具函数** | [jsonutil](./jsonutil/) | JSON 操作工具（基于 gjson/sjson） |
| **工具函数** | [pointer](./pointer/) | 值与指针互转的泛型辅助函数 |
| **工具函数** | [times](./times/) | 时间处理工具 |
| **工具函数** | [jodatime](./jodatime/) | Joda 风格日期格式与 Go 格式互转 |
| **工具函数** | [idgen](./idgen/) | 分布式 ID 生成器（雪花算法） |
| **工具函数** | [page](./page/) | 分页参数计算 |
| **内部组件** | [bufferx](./bufferx/) | 多规格 Buffer 池，基于 sync.Pool 减少 GC 压力 |
| **内部组件** | [syncer](./syncer/) | 线程安全的 WriteSyncer 抽象，支持多路写入 |
| **内部组件** | [errors](./errors/) | 自定义错误体系，支持错误码和堆栈追踪 |

## 快速开始

### 配置文件

默认读取 `./conf/application.yaml`：

```yaml
application:
  name: my_server
  port: 8080
debug: true

database:
  db_type: "mysql"
  dsn: "root:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conn: 10
  max_open_conn: 100

redis:
  endpoints:
    - "127.0.0.1:6379"
  password: ""
  index_db: 0

log:
  write_file: true
  json_log: false
  log_level: debug
  log_file_conf:
    log_file_path: ./log/server.log
    max_size: 10
    max_age: 28
    max_backups: 100
    compress: false
```

### 日志使用

```go
import "github.com/jasonlabz/potato/log"
import "github.com/jasonlabz/potato/log/zapx"

// 方式一：导入包自动初始化
import _ "github.com/jasonlabz/potato/log/zapx"

// 方式二：手动初始化，指定上下文字段
zapx.InitLogger(
    zapx.WithConfigPath("conf/application.yaml"),
    zapx.WithFields("user_id", "trace_id"),
)

// 使用
logger := log.GetLogger()
logger.Info(ctx, "request received", "key", "value")
```

### Redis 使用

```go
import "github.com/jasonlabz/potato/goredis"

op, err := goredis.NewRedisOperator(&goredis.Config{
    Addrs:    []string{"127.0.0.1:6379"},
    Password: "xxx",
})
op.Set(ctx, "key", "value")
val, _ := op.Get(ctx, "key")
```

### RabbitMQ 使用

```go
import "github.com/jasonlabz/potato/rabbitmqx"

operator, _ := rabbitmqx.NewRabbitMQOperator(&rabbitmqx.MQConfig{
    Username: "guest", Password: "guest",
    Host: "127.0.0.1", Port: 5672,
})
// 推送消息
msg := &rabbitmqx.PushBody{}
operator.Push(ctx, msg.SetMsg([]byte("hello")).
    SetExchangeName("exchange").
    BindQueue("queue", "routing_key"))
// 消费消息
body := &rabbitmqx.ConsumeBody{}
deliveries, _ := operator.Consume(ctx, body.SetQueueName("queue"))
for d := range deliveries {
    fmt.Println(string(d.Body))
    operator.Ack(ctx, d)
}
```

## 项目结构

```
potato/
├── bloom/          # 布隆过滤器
├── bufferx/        # Buffer 对象池
├── configx/        # 配置管理（File/ETCD）
├── consts/         # 全局常量
├── cron/           # 定时任务
├── cryptox/        # 加解密（AES/DES/RSA/HMAC/MD5/SHA）
├── errors/         # 错误体系
├── es/             # Elasticsearch 客户端
├── ginmetrics/     # Gin Prometheus 指标
├── ginx/           # Gin 响应封装
├── gocache/        # 本地内存缓存
├── goredis/        # Redis 客户端（Single/Cluster/Sentinel）
├── gormx/          # GORM 数据库封装
├── httpx/          # HTTP 客户端
├── idgen/          # 分布式 ID 生成
├── internal/       # 内部接口定义
├── jodatime/       # Joda 日期格式转换
├── jsonutil/       # JSON 工具
├── jwtx/           # JWT 认证
├── kafkax/         # Kafka 客户端
├── kube/           # Kubernetes 客户端
├── log/            # Zap 日志封装
├── middleware/     # Gin 中间件
├── mongo/          # MongoDB 客户端
├── page/           # 分页工具
├── pointer/        # 指针转换工具
├── poolx/          # 协程池
├── rabbitmqx/      # RabbitMQ 客户端
├── ral/            # 服务调用抽象层
├── rocketmqx/      # RocketMQ 客户端
├── slogx/          # Slog 日志封装
├── streams/        # 函数式流处理
├── stringutil/     # 字符串工具
├── syncer/         # IO 同步写入
├── times/          # 时间工具
└── utils/          # 通用工具
```

## 环境要求

- Go 1.24+
- 各中间件组件按需安装对应服务

## License

MIT
