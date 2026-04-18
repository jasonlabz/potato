# bufferx

基于 `sync.Pool` 的多规格 Buffer 对象池，减少频繁创建 `bytes.Buffer` 带来的 GC 压力。

## 核心类型

- `BufferPool` - Buffer 池接口，提供 `Get()` 和 `Put()` 方法
- `Once` / `OncePointer` - 扩展的单次初始化原语，存储计算结果，避免额外变量

## 预定义池规格

提供 7 种预分配大小的全局 Buffer 池，按需懒加载：

| 函数 | 初始容量 |
|------|---------|
| `GetBuff64()` | 64 字节 |
| `GetBuff128()` | 128 字节 |
| `GetBuff512()` | 512 字节 |
| `GetBuff1024()` | 1 KB |
| `GetBuff2048()` | 2 KB |
| `GetBuff4096()` | 4 KB |
| `GetBuff8192()` | 8 KB |

## 使用示例

```go
// 获取预定义池的 Buffer
pool := bufferx.GetBuff1024()
buf := pool.Get()
defer pool.Put(buf)

buf.WriteString("hello")
// 使用 buf...

// 自定义大小
customPool := bufferx.NewBuffer(16384)
buf2 := customPool.Get()
defer customPool.Put(buf2)
```
