# cron

基于 [robfig/cron](https://github.com/robfig/cron) 的定时任务封装，支持秒级精度和 Fluent Builder 风格构建 Cron 表达式。

## 核心接口

```go
// base/base.go
type JobBase interface {
    GetJobName() string
    Run()
}
```

## 使用示例

### 初始化

```go
import "github.com/jasonlabz/potato/cron"

// 标准模式（5 字段：分 时 日 月 周）
cron.InitCron()

// 秒级模式（6 字段：秒 分 时 日 月 周）
cron.InitCron(cron.WithParseTypeSecond())
```

### 注册任务

```go
// 注册 JobBase 实例
cron.RegisterJob("0 */5 * * *", myJob, false)

// 注册函数
cron.RegisterFunc("@every 30s", func() {
    fmt.Println("tick")
}, false)

// runFirst=true 表示注册时立即执行一次
cron.RegisterJob("0 0 * * *", dailyJob, true)
```

### Fluent Builder 构建 Cron 表达式

```go
builder := &cron.CronTaskBuilder{}
spec := builder.
    SetMinutes("0").
    SetHours("*/2").
    BuildSpec() // "0 */2 * * *"

// 或使用预设任务类型
spec = cron.GenCrontabStr(cron.IntervalJob) // "@every 5m"
```

### 停止任务

```go
cron.RemoveCronJob("job_name")
ctx := cron.StopCron() // 优雅停止
```
