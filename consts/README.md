# consts

全局常量定义，包括上下文键名、HTTP Header、标点符号、数字常量和 API 版本号。

## 上下文与 Header 常量（context.go）

```go
// HTTP Header
HeaderRequestID     = "X-RequestID"
HeaderAuthorization = "X-Authorization"
HeaderUserID        = "X-UserID"
HeaderRemote        = "X-Client"

// Context Key（用于日志追踪、用户身份等）
ContextLOGID      = "log_id"
ContextTraceID    = "trace_id"
ContextUserID     = "user_id"
ContextSpanID     = "span_id"
ContextToken      = "token"
ContextClientAddr = "client_ip"
```

## 标点与符号常量（mark.go）

提供 `EmptyString`、`SignDash`、`SignDot`、`SignComma`、`SignSlash`、`SignVertical`、`SignUnderline`、`SignStar`、`SignColon` 等常用符号常量，以及 `NumberZero` ~ `NumberTen` 数字常量。

## 其他常量

- `DefaultConfigPath = "./conf/application.yaml"`（default.go）
- `APIVersionV1` ~ `APIVersionV4`（version.go）
