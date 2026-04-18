# errors

自定义错误体系，支持错误码、错误消息和堆栈追踪，基于 `github.com/pingcap/errors` 和 `github.com/pkg/errors`。

## 核心类型

```go
type ErrInfo struct {
    Code    int    // 错误码
    Message string // 错误消息
}
```

## 预定义错误码

包含常见 HTTP/业务错误码：
- `ErrCodeSuccess` (0) - 成功
- `ErrCodeBadRequest` (400) - 请求错误
- `ErrCodeUnauthorized` (401) - 未授权
- `ErrCodeForbidden` (403) - 禁止访问
- `ErrCodeNotFound` (404) - 资源不存在
- `ErrCodeInternalError` (500) - 服务器内部错误
- 其他业务自定义错误码

## 使用示例

```go
import "github.com/jasonlabz/potato/errors"

// 创建错误
err := errors.New("something went wrong")

// 带错误码
err = errors.NewWithCode(500, "internal error")

// 包装错误（保留堆栈）
err = errors.Wrap(originalErr, "additional context")

// 获取错误码
code := errors.GetCode(err)
```
