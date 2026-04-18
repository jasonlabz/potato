# jsonutil

基于 [gjson](https://github.com/tidwall/gjson) 和 [sjson](https://github.com/tidwall/sjson) 的 JSON 操作工具，支持路径查询和动态修改。

## 核心类型

- `JSON` - JSON 文档包装器，支持链式路径查询和修改

## 使用示例

```go
import "github.com/jasonlabz/potato/jsonutil"

// 从字符串创建
j := jsonutil.NewFromString(`{"name":"张三","age":30}`)

// 路径查询
name := j.Get("name").String()  // "张三"
age := j.Get("age").Int()       // 30

// 嵌套路径
city := j.Get("address.city").String()

// 数组访问
first := j.Get("items.0").String()

// 修改值
j.Set("name", "李四")
j.Set("address.city", "北京")

// 序列化/反序列化
jsonStr := j.String()
```

## 特性

- 基于路径的 JSON 查询（支持嵌套、数组下标）
- 动态 JSON 修改
- 无需预定义结构体
