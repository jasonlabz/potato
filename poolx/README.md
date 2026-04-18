# poolx

基于 [ants](https://github.com/panjf2000/ants) 的协程池封装，用于限制并发 Goroutine 数量。

## 使用示例

```go
import "github.com/jasonlabz/potato/poolx"

// 创建固定大小的协程池
pool, err := poolx.GetFixedPool(100)
defer pool.Release()

// 提交任务到默认池
poolx.Submit(func() {
    // 执行异步任务
})
```

## 特性

- 固定大小的 Goroutine 池，防止 Goroutine 泄漏
- 任务队列，超出池大小时排队等待
- 基于 ants v2，高性能低内存
