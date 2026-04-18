# page

简单的分页参数计算工具。

## 核心类型

```go
type Pagination struct {
    Page      int64 // 当前页码
    PageSize  int64 // 每页大小
    PageCount int64 // 总页数
    Total     int64 // 总记录数
}
```

## 使用示例

```go
import "github.com/jasonlabz/potato/page"

p := &page.Pagination{
    Page:     1,
    PageSize: 20,
    Total:    100,
}

// 计算总页数
pageCount := p.GetPageCount() // 5

// 计算 SQL OFFSET
offset := p.GetOffset() // 0
```
