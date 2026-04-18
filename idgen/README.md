# idgen

分布式 ID 生成器，基于 [yitter/idgenerator-go](https://github.com/yitter/idgenerator-go) 雪花算法实现。

## 使用示例

```go
import "github.com/jasonlabz/potato/idgen"

// 初始化（设置 WorkerID）
idgen.Init(1)

// 生成唯一 ID
id := idgen.NextID()
```

## 特性

- 基于雪花算法，生成趋势递增的唯一 ID
- 支持多 Worker 节点部署
- 高性能，线程安全
