# bloom

布隆过滤器（Bloom Filter）实现，用于快速判断元素是否存在于集合中，具有低误判率的特性。

## 核心类型

- `BloomFilter` - 布隆过滤器，内部维护一个 BitSet 和 6 个独立哈希函数

## 主要函数

```go
// 创建布隆过滤器（默认 2^25 位，6 个哈希种子）
filter := bloom.NewBloomFilter()

// 添加元素
filter.Add("hello")

// 查询元素是否存在（可能误判，不会漏判）
exists := filter.Contains("hello") // true
exists = filter.Contains("world")  // false
```

## 参数说明

- 默认位数组大小：`2 << 24`（33,554,432 位）
- 哈希种子：`[7, 11, 13, 31, 37, 61]`，共 6 个独立哈希函数
- 空字符串调用 `Contains` 固定返回 `false`
