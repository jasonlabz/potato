# ral

HTTP 请求抽象层（Resource Access Layer），用于服务间 HTTP 调用。

通过服务名（servicer 配置）自动获取客户端实例，无需手动管理 baseURL、超时、重试等配置。
gRPC 服务请使用 `ral.GetServiceConn()` 获取连接后通过生成的 stub 调用。

## 核心类型

- `Request` - HTTP 请求抽象，包含 Service、Body、Headers、Query、Path 等
- `Response` - HTTP 响应封装
- `RALOption` - 函数式选项（WithBody/WithHeaders/WithQuery/WithToken/WithCookies）

## 使用示例

### 1. RAL 通用方法（最灵活）

```go
import "github.com/jasonlabz/potato/ral"

var result MyResponseType

// 基本调用：通过服务名 + HTTP 方法 + 路径
err := ral.RAL(ctx, "dagine_engine", "POST", "/scheduler/job/submit", &result,
    ral.WithBody(reqBody),
    ral.WithHeader("X-Custom", "value"),
)

// GET 请求带查询参数
err := ral.RAL(ctx, "dagine_engine", "GET", "/operators", &result,
    ral.WithQueryParam("kind", "reader"),
)
```

### 2. 便捷方法（常用场景）

```go
// GET
err := ral.Get(ctx, "dagine_engine", "/operators", &result)

// POST（body 自动设置）
err := ral.Post(ctx, "dagine_engine", "/scheduler/job/submit", reqBody, &result)

// PUT
err := ral.Put(ctx, "dagine_engine", "/resource/1", updateBody, &result)

// PATCH
err := ral.Patch(ctx, "dagine_engine", "/resource/1", patchBody, &result)

// DELETE
err := ral.Delete(ctx, "dagine_engine", "/resource/1", &result)

// POST Form
err := ral.PostForm(ctx, "dagine_engine", "/login", formData, &result)

// Multipart 文件上传
err := ral.PostMultipart(ctx, "dagine_engine", "/upload", files, formFields, &result)
```

### 3. Request 高级用法

```go
resp, err := ral.Do(ctx, &ral.Request{
    Service: "dagine_engine",
    Method:  http.MethodPost,
    Path:    "/scheduler/job/submit",
    Header:  http.Header{"X-Custom": []string{"value"}},
    Query:   url.Values{"detail": []string{"true"}},
    Body:    bytes.NewReader(jsonData),
    Result:  &result,
})
```

### 4. gRPC 服务调用

gRPC 服务不能通过 `RAL()` 调用，请通过 `GetServiceConn()` 获取连接后使用生成的 stub：

```go
// 获取 gRPC 连接
conn := ral.GetServiceConn("user_service")

// 使用生成的 stub 调用（类型安全）
client := pb.NewUserServiceClient(conn.Conn())
resp, err := client.GetUser(ctx, &pb.GetUserReq{Id: 123})
```

对应的 `conf/servicer/user_service.yaml` 配置：

```yaml
Name: user_service
Protocol: grpc
Host: 127.0.0.1
Port: 9000
Timeout: 10000
```

## 可选参数

| 选项 | 说明 |
|------|------|
| `WithBody(body)` | 设置请求体 |
| `WithHeaders(map)` | 批量设置请求头 |
| `WithHeader(k, v)` | 设置单个请求头 |
| `WithQuery(url.Values)` | 批量设置查询参数 |
| `WithQueryParam(k, v)` | 设置单个查询参数 |
| `WithToken(token)` | 设置 Bearer Token |
| `WithCookies(cookies)` | 设置 Cookies |

## 特性

- 统一的服务调用 API，通过服务名自动获取客户端实例
- 自动协议路由：HTTP 服务走 httpx，gRPC 服务返回友好错误提示
- 自动序列化/反序列化（JSON）
- 集成日志和上下文传递
- 支持查询参数自动拼接到 URL
- 支持请求头、Token、Cookies 等透传
- 支持 Form 和 Multipart 上传
