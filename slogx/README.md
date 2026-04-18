# slogx

基于 Go 标准库 `log/slog` 的日志封装，提供与 `log` 包一致的上下文字段提取能力。

## 使用示例

```go
import "github.com/jasonlabz/potato/slogx"

logger := slogx.GetLogger()
logger.Info(ctx, "message", "key", "value")
logger.Error(ctx, "error occurred", "err", err)
```

## 特性

- 自动从 Context 提取 trace_id、user_id 等字段
- 与 `log` 包 API 风格一致
- 适用于不需要 Zap 依赖的轻量场景
