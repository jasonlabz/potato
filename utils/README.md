# utils

通用工具函数集合，提供类型转换、随机数生成、结构体操作等常用功能。

## 主要函数

- **随机数生成**：`RandLowercase(n)` / `RandUppercase(n)` / `RandNumeric(n)` - 生成指定长度的随机字符串
- **类型转换**：`StringValue(ptr)` / `IntValue(ptr)` - 安全的指针取值
- **结构体操作**：`CopyStruct(src, dst)` - 结构体字段拷贝
- **切片操作**：去重、查找、分组等
- **UUID 生成**：基于 `google/uuid`
