# ginx

Gin 框架的标准化 HTTP 响应封装，统一接口返回格式。

## 响应结构

```json
{
  "code": 0,
  "message": "success",
  "err_trace": "",
  "version": "v1",
  "current_time": "2024-01-01 12:00:00",
  "data": []
}
```

## 核心类型

- `Response` - 标准响应结构体
- `ResponseWithPagination` - 带分页信息的响应结构体

## 使用示例

```go
import "github.com/jasonlabz/potato/ginx"

func Handler(c *gin.Context) {
    // 成功响应
    ginx.ResponseOK(c, data)

    // 错误响应
    ginx.ResponseErr(c, err)

    // 带分页
    ginx.PaginationResult(c, list, pagination, err)

    // 通用（根据 err 自动判断成功/失败）
    ginx.JsonResult(c, data, err)
}
```

## 特性

- 自动将非数组数据包装为数组格式
- 自动从 error 中提取错误码和错误消息
- 响应中自动附带 API 版本号和当前时间
- 错误响应自动记录日志
