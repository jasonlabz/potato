# gocache

基于 [go-cache](https://github.com/patrickmn/go-cache) 的本地内存缓存封装，提供简洁的全局缓存 API。

## 默认参数

- 默认过期时间：5 分钟
- 清理间隔：20 分钟

## 使用示例

```go
import "github.com/jasonlabz/potato/gocache"

// 设置缓存（带 TTL）
gocache.Set("key", "value", 10*time.Minute)

// 读取缓存
val, found := gocache.Get("key")

// 检查是否存在
exists := gocache.IsExist("key")

// 删除
gocache.Del("key")

// 替换（key 必须已存在）
gocache.Replace("key", "new_value", 5*time.Minute)

// 原子递增/递减
gocache.IncrementInt("counter", 1)
gocache.DecrementInt("counter", 1)

// 永不过期
gocache.Set("permanent", data, gocache.NoExpiration)
```
