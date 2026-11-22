# log

基于 [Zap](https://github.com/uber-go/zap) 的日志封装，支持上下文字段自动提取、文件轮转和分级输出。

## 目录结构

```
log/
├── wrapper.go   # LoggerWrapper 通用接口、字段归一化、全局实例
├── zap.go       # Zap 初始化与配置加载
└── logger.yaml  # 配置示例
```

## 核心特性

- 自动从 Context 中提取 `trace_id`、`user_id` 等字段
- 基于 Lumberjack 的日志文件轮转
- 高低级别分文件输出（ERROR 及以上单独写入 `.wf` 文件）
- JSON / 文本两种输出格式
- 字段值保留原始类型（数字、布尔不会被转成字符串）
- 支持配置文件和代码两种初始化方式

## 使用示例

### 初始化

```go
// 方式一：直接用默认配置（首次 GetLogger 时自动加载 conf/logger.yaml 等默认路径）
logger := log.GetLogger()

// 方式二：显式初始化并安装为全局实例
log.SetLogger(log.NewLogger(
    log.WithConfigPath("conf/logger.yaml"),
    log.WithLevel("debug"),
    log.WithLogFields("user_id", "trace_id"),
))
```

### 使用

```go
import "github.com/jasonlabz/potato/log"

logger := log.GetLogger()
logger.Info(ctx, "request received", "key", "value")
logger.Error(ctx, "something failed", "err", err)
logger.WithError(err).Error(ctx, "detailed error")
logger.WithField(log.String("extra", "field")).Info(ctx, "with fields")
```

字段参数支持三种写法且可以混用：`log.String(...)` 这类 `zap.Field`、`"key", value` 键值对、以及直接传 `error`。
键值对的值会保留原始类型；参数不成对或 key 不是字符串时，会以 `!BADKEY` 字段输出，便于发现调用错误。

### 创建独立 Logger

```go
customLogger := log.NewLogger(
    log.WithWriteFile(true),
    log.WithBasePath("./log/custom/"),
    log.WithFileName("custom.log"),
    log.WithLogField("trace_id"),
)
customLogger.Info(ctx, "custom log message")
```

### 配置文件

```yaml
# 是否写入文件
name: service
# json|console
format: console
# error|warn|info|debug|fatal
log_level: debug
# 文件配置
write_file: true
# 日志文件路径
base_path: log
# 日志文件大小
max_size: 10
# 日志文件最大天数
max_age: 28
# 最大存在数量
max_backups: 100
# 是否压缩日志
compress: false
```

### 日志输出效果

```
2024-01-07 16:08:10.147852  INFO  middleware/logger.go:68  [GIN] request  {"trace_id": "33a5fb6c...", "remote_addr": "127.0.0.1", "method": "GET"}
```
