# httpx

基于 [resty](https://github.com/go-resty/resty) 的 HTTP 客户端封装，提供简洁的 RESTful 请求方法。

## 使用示例

```go
import "github.com/jasonlabz/potato/httpx"

client := httpx.GetClient()

// GET 请求
resp, err := client.Get(ctx, "https://api.example.com/users")

// POST JSON
resp, err = client.Post(ctx, "https://api.example.com/users", body)

// POST Form
resp, err = client.PostForm(ctx, "https://api.example.com/login", formData)

// PUT / PATCH / DELETE
resp, err = client.Put(ctx, url, body)
resp, err = client.Patch(ctx, url, body)
resp, err = client.Delete(ctx, url, body)
```

## 特性

- 全局单例客户端
- 支持自定义 Header 和 Query 参数
- 内置请求/响应日志记录
- 支持 JSON 和 Form 两种请求体格式

## 按调用方创建客户端

`Config.Name` 供 ral 按服务名取客户端，非空时必须全局唯一。Name 留空即匿名客户端，不登记重名，
适合每个调用方（例如每个算子实例）各自创建一个，超时、重试、TLS、认证互不影响，用完调用 `Close`：

```go
client := httpx.NewHttpClient(&httpx.Config{
	Protocol:        "https",
	Endpoint:        "https://api.example.com",
	Timeout:         30000,
	DisableBodyLog:  true, // 不记录请求体、表单与响应体
	DebugSuccessLog: true, // 成功日志记 Debug，失败仍为 Error
})
defer client.Close()

resp, err := client.Do(ctx, http.MethodGet, "/items", nil, &out)
if err == nil && resp.IsError() {
	// 非 2xx
}
```
