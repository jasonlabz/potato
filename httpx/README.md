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
