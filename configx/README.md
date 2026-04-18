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

## 配置校验

`Validate()` 方法会检查：
- Application.Name 不能为空
- HTTP/GRPC 端口范围 1~65535
