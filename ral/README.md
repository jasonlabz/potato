# ral

HTTP 请求抽象层（Resource Access Layer），用于服务间 HTTP 调用。

## 核心类型

- `RalRequest` - HTTP 请求抽象，包含 Body、Headers、Query、URL 等

## 使用示例

```go
import "github.com/jasonlabz/potato/ral"

var result ResponseType
err := ral.RAL(ctx, "service_name", "POST", "/api/endpoint", requestBody, &result)
```

## 特性

- 统一的服务调用 API
- 自动序列化/反序列化
- 集成日志和上下文传递
