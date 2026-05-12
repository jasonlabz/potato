# configx

多格式、多来源的配置管理系统，支持文件配置和 ETCD 远程配置，提供热更新能力。

## 架构设计

```
configx/
├── base/     # Provider 接口定义
├── file/     # 文件配置 Provider（基于 Viper + fsnotify）
├── etcd/     # ETCD 远程配置 Provider
├── config.go # 核心配置结构体和全局 API
├── extra.go  # 辅助函数（工作目录、环境变量等）
└── server.go # 服务器配置结构体
```

## 支持的配置格式

- YAML / YML
- JSON
- TOML
- INI
- Properties
- ENV

## 核心类型

- `Config` - 全局配置结构体，包含 Application、DataSource、Redis、Kafka、RabbitMQ、RocketMQ、ES、MongoDB、Crypto 等子配置
- `IProvider` - 配置提供者接口，定义 `Get(key)` 和 `IsExist(key)` 方法
- `ProviderManager` - 管理多个 Provider 实例

## 使用示例

### 文件配置

```go
import "github.com/jasonlabz/potato/configx/file"

// 创建文件配置 Provider（自动监听文件变更）
provider := file.NewConfigProvider("./conf/application.yaml")

// 通过全局 Search API 查询
val := configx.SearchString("database.db_type")
port := configx.SearchInt("application.port")
```

### ETCD 远程配置

```go
import "github.com/jasonlabz/potato/configx/etcd"

provider := etcd.NewConfigProvider(
    "http://localhost:2379",
    "/config/app",
    "yaml",
)
```

### 全局配置结构体

```go
// 自动从 ./conf/application.yaml 加载
cfg := configx.GetConfig()
fmt.Println(cfg.Application.Name)
fmt.Println(cfg.DataSource.DBType)
```

### 辅助函数

```go
configx.Pwd()              // 获取工作目录
configx.ConfDir()           // 获取配置目录（./conf 或 ./config）
configx.Store("key", val)   // 全局键值存储
configx.LoadKey("key")      // 全局键值读取
configx.GetEnv("ENV_NAME")  // 读取环境变量
```

## YAML 配置参考

```yaml
application:
  name: myapp
  debug: false
  server:
    http:
      port: 8080
      read_timeout: 30s
      write_timeout: 30s
    grpc:
      port: 8082
      max_concurrent_streams: 100
  monitor:
    prometheus:
      enable: false
      path: "metrics"
      scrape_interval: "15s"
    pprof:
      enable: false
      port: 8080
      enabled_endpoints: ["goroutine", "heap"]

datasource:
  enable: true
  strict: true
  db_type: "postgres"
  host: "127.0.0.1"
  port: 5432
  username: postgres
  password: "******"
  database: mydb
  args:
    - name: sslmode
      value: disable
    - name: TimeZone
      value: Asia/Shanghai
  log_mode: "info"
  max_idle_conn: 10
  max_open_conn: 100

redis:
  enable: false
  strict: true
  endpoints:
    - "127.0.0.1:6379"
  password: "******"
  index_db: 0
  min_idle_conns: 10
  max_idle_conns: 50
  max_active_conns: 100
  max_retry_times: 5

rabbitmq:
  enable: false
  strict: true
  host: "127.0.0.1"
  port: 5672
  username: guest
  password: "******"
  limit_conf:
    attempt_times: 3
    retry_wait_time: 3000
    prefetch_count: 100
    timeout: 5000

kafka:
  enable: false
  strict: true
  topic: ["my-topic"]
  group_id: "my-group"
  bootstrap_servers: ["127.0.0.1:9092"]
  security_protocol: "PLAINTEXT"
  sasl_mechanism: "PLAIN"

rocketmq:
  enable: false
  strict: true
  name_servers: ["127.0.0.1:9876"]
  group_name: "my-group"
  access_key: "ak"
  secret_key: "sk"

es:
  enable: false
  strict: true
  endpoints: ["127.0.0.1:9200"]
  username: elastic
  password: "******"
  is_https: false
  insecure_skip_verify: false

mongodb:
  enable: false
  strict: true
  host: "127.0.0.1"
  port: 27017
  username: admin
  password: "******"
  max_pool_size: 100

obs:
  enable: false
  strict: false
  name: "minio-dev"                                     # client name, for multi-client lookup
  endpoint: "127.0.0.1:9000"                            # MinIO / OBS / S3-compatible endpoint
  region: "us-east-1"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket: "my-bucket"
  insecure_skip_verify: true                             # true when using HTTP or self-signed cert
  max_retries: 3
  timeout: 30                                            # seconds

crypto:
  - type: aes
    key: "your-aes-key-16byte"
  - type: des
    key: "des-key8"
```

## 配置校验

`Validate()` 方法会检查：
- Application.Name 不能为空
- HTTP/GRPC 端口范围 1~65535
