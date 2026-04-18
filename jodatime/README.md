# jodatime

Joda 风格日期格式与 Go 标准日期格式之间的互转工具。

## 格式对照

| Joda 格式 | Go 格式 | 含义 |
|-----------|---------|------|
| `yyyy` | `2006` | 四位年份 |
| `MM` | `01` | 两位月份 |
| `dd` | `02` | 两位日期 |
| `HH` | `15` | 24 小时制 |
| `mm` | `04` | 分钟 |
| `ss` | `05` | 秒 |

## 使用示例

```go
import "github.com/jasonlabz/potato/jodatime"

// Joda 格式 → Go 格式
goFmt := jodatime.ToGoFormat("yyyy-MM-dd HH:mm:ss")
// goFmt = "2006-01-02 15:04:05"

// 使用 Joda 格式解析时间
t, _ := jodatime.Parse("yyyy-MM-dd", "2024-01-15")

// 使用 Joda 格式格式化时间
str := jodatime.Format("yyyy-MM-dd HH:mm:ss", time.Now())
```
