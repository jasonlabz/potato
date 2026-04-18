# streams

函数式流处理库，提供类似 Java Stream 的链式操作 API。

## 主要操作

- `Map` - 元素映射转换
- `Filter` - 条件过滤
- `Reduce` - 聚合归约
- `Sort` - 排序
- `Distinct` - 去重
- `Limit` / `Skip` - 分页截取
- `FlatMap` - 扁平化映射
- `ForEach` - 遍历
- `Count` - 计数
- `AnyMatch` / `AllMatch` - 匹配判断
- `Collect` - 收集结果

## 使用示例

```go
import "github.com/jasonlabz/potato/streams"

// 过滤 + 映射 + 收集
result := streams.Of(1, 2, 3, 4, 5).
    Filter(func(i int) bool { return i > 2 }).
    Map(func(i int) int { return i * 2 }).
    Collect()
// result: [6, 8, 10]

// 去重 + 排序
result = streams.Of(3, 1, 2, 1, 3).
    Distinct().
    Sort(func(a, b int) bool { return a < b }).
    Collect()
// result: [1, 2, 3]
```
