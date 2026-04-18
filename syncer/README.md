# syncer

线程安全的 WriteSyncer 抽象，用于日志和文件 IO 的同步写入，支持多路写入。

## 核心类型

- `WriteSyncer` - 接口，组合 `io.Writer` 和 `Sync()` 方法
- `lockedWriteSyncer` - 带 Mutex 的线程安全 WriteSyncer
- `multiWriteSyncer` - 多目标同时写入（类似 io.MultiWriter）
- `FileWriter` - 基于 `*os.File` 的 WriteSyncer 实现

## 使用示例

```go
import "github.com/jasonlabz/potato/syncer"

// 将 io.Writer 转换为 WriteSyncer
ws := syncer.AddSync(writer)

// 加锁保护
locked := syncer.Lock(ws)

// 多路写入
multi := syncer.NewMultiWriteSyncer(ws1, ws2, ws3)

// 文件写入
fw, err := syncer.NewFileWriter("./log/output.log")
defer fw.Close()
fw.Write([]byte("log line\n"))
fw.Sync()
```

## 特性

- 线程安全（Mutex 保护）
- 防重复包装（Lock 检测已加锁的 WriteSyncer）
- 多路写入错误聚合（使用 `go.uber.org/multierr`）
- 文件以追加模式打开（0644 权限）
