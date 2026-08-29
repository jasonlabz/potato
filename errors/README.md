# errors

`errors` 提供可注册的应用错误。它负责保存稳定错误码、对外文案和内部根因；HTTP 状态、响应信封和日志策略由应用自身的 transport 层决定。

## 使用规则

- `0` 仅表示成功，不能用于任何错误。
- 一个错误码只能注册一次。`New` 适合包级 `var` 初始化，不能在请求路径中调用。
- 对外错误码与 HTTP 状态分离。HTTP 状态表达协议结果，应用错误码表达调用方可识别的稳定语义。
- 对外文案必须安全、可展示；数据库、连接串、token、堆栈等根因只能通过 `WithErr` 保存到服务端日志或受控的开发诊断信息中。
- 已发布的错误码不得复用。需要废弃时保留原定义并停止产生它。

## 推荐的九位应用错误码

应用可采用 `SSMMTTDDD` 格式：

```text
SS      系统标识，由组织统一分配
MM      业务模块
TT      调用方可感知的错误类别
DDD     该类别中的具体错误序号
```

建议让类别表达稳定的处理语义，例如参数校验、认证、权限、资源不存在、状态冲突、业务限制、依赖不可用、内部错误或限流；不要以数据库、缓存或某个 Client 等可替换实现细节作为对外码分类。

库内的 `NormalErrorCode`、`UndefinedErrorCode` 和 `ErrUndefined` 是通用库兜底值，不属于任何应用的九位目录，应用不应将它们直接作为公开 API 错误码。

## 定义与包装

```go
package apperr

import potatoErrors "github.com/jasonlabz/potato/errors"

var ErrDataSourceUnavailable = potatoErrors.New(990307001, "数据源暂不可用，请稍后重试")

func Connect(err error) error {
	return ErrDataSourceUnavailable.WithErr(err)
}
```

`Message()` 只返回安全文案；`Error()` 包含错误链，适合日志和仅限开发环境的诊断字段。`Cause()` 返回最深层原因，`Unwrap()` 支持标准库 `errors.Is` 和 `errors.As`。

## 查询注册项

`GetError` 找不到编码时返回稳定的 `ErrUndefined`，不会重复注册或 panic。调用方应把它视为未识别输入，而不是将其直接返回给 API 使用者。
