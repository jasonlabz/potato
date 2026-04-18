# log

基于 [Zap](https://github.com/uber-go/zap) 的日志封装，支持上下文字段自动提取、文件轮转和分级输出。

## 目录结构

```
log/
├── wrapper.go    # LoggerWrapper 通用接口
└── zapx/
    └── zap.go    # Zap 实现，配置和初始化
```

## 核心特性

- 自动从 Context 中提取 `trace_id`、`user_id`、`span_id` 等字段
- 基于 Lumberjack 的日志文件轮转
- 高低级别分文件输出（ERROR 以上单独文件）
- JSON / 文本两种输出格式
- 支持配置文件和代码两种初始化方式

## 使用示例

### 初始化

```go
// 方式一：导入包自动初始化（使用默认配置）
import _ "github.com/jasonlabz/potato/log/zapx"

// 方式二：手动初始化
import "github.com/jasonlabz/potato/log/zapx"

zapx.InitLogger(
    zapx.WithConfigPath("conf/zap.yaml"),
    zapx.WithFields("user_id", "trace_id"),
)
```

### 使用

```go
import "github.com/jasonlabz/potato/log"

logger := log.GetLogger()
logger.Info(ctx, "request received", "key", "value")
logger.Error(ctx, "something failed", "err", err)
logger.WithError(err).Error(ctx, "detailed error")
logger.WithField(zap.String("extra", "field")).Info(ctx, "with fields")
```

### 创建独立 Logger

```go
customLogger := zapx.NewLogger(
    zapx.WithWriteFile(true),
    zapx.WithBasePath("./log/custom/"),
    zapx.WithFileName("custom.log"),
    zapx.WithLogField("trace_id"),
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
