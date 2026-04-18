# pointer

值与指针之间的转换辅助函数，覆盖 Go 所有内置类型，支持单值、切片和 Map 转换。

## 支持的类型

`string`, `bool`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `time.Time`

## 使用示例

```go
import "github.com/jasonlabz/potato/pointer"

// 值 → 指针
p := pointer.String("hello")      // *string
p2 := pointer.Int64(42)           // *int64
p3 := pointer.Bool(true)          // *bool
p4 := pointer.Time(time.Now())    // *time.Time

// 指针 → 值（nil 安全，返回零值）
s := pointer.StringValue(p)       // "hello"
n := pointer.Int64Value(nil)      // 0

// 切片转换
ptrs := pointer.StringSlice([]string{"a", "b"})      // []*string
vals := pointer.StringValueSlice(ptrs)                 // []string

// Map 转换
ptrMap := pointer.StringMap(map[string]string{"k": "v"})     // map[string]*string
valMap := pointer.StringValueMap(ptrMap)                       // map[string]string
```
