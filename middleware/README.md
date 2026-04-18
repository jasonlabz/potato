# middleware

Gin 框架中间件集合，提供鉴权、上下文注入、请求日志和 Panic 恢复功能。

## 中间件列表

### AuthCheck - 鉴权中间件

从请求 Header 中提取 JWT Token，验证后将用户信息注入 Context。

```go
r.Use(middleware.AuthCheck())
```

### SetContext - 上下文注入中间件

从 HTTP Header 中提取关键字段（如 X-RequestID、X-UserID、X-Client）注入到 Gin Context 中，生成 trace_id 和 log_id 用于日志追踪。

```go
r.Use(middleware.SetContext())
```

### RequestMiddleware - 请求日志中间件

记录每个 HTTP 请求的详细信息（路径、方法、参数、响应状态、耗时等）。

```go
r.Use(middleware.RequestMiddleware())
```

### RecoveryLog - Panic 恢复中间件

捕获 Handler 中的 Panic，记录堆栈信息并返回 500 响应，防止服务崩溃。

```go
r.Use(middleware.RecoveryLog(true)) // true 表示记录堆栈
```

## 推荐使用顺序

```go
r := gin.New()
r.Use(middleware.RecoveryLog(true))
r.Use(middleware.SetContext())
r.Use(middleware.RequestMiddleware())
r.Use(middleware.AuthCheck()) // 按需使用
```
