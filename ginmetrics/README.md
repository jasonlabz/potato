# ginmetrics

Gin 框架的 Prometheus 指标采集中间件，自动收集 HTTP 请求的性能和流量指标。

## 默认采集指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `gin_request_total` | Counter | 请求总数 |
| `gin_request_uv` | Counter | 独立访客数（基于 IP） |
| `gin_uri_request_total` | Counter | 按 URI/Method/StatusCode 分类的请求数 |
| `gin_request_body_total` | Counter | 请求体总字节数 |
| `gin_response_body_total` | Counter | 响应体总字节数 |
| `gin_request_duration` | Histogram | 请求处理耗时分布 |
| `gin_slow_request_total` | Counter | 慢请求计数（默认阈值 5 秒） |

## 使用示例

```go
import "github.com/jasonlabz/potato/ginmetrics"

r := gin.Default()

// 获取全局 Monitor 实例
m := ginmetrics.GetMonitor()

// 可选：设置指标暴露路径（默认 /debug/metrics）
m.SetMetricPath("/metrics")
// 可选：设置慢请求阈值（默认 5 秒）
m.SetSlowTime(10)
// 可选：设置耗时分布桶（用于 p95, p99）
m.SetDuration([]float64{0.1, 0.3, 1.2, 5, 10})

// 注册中间件并暴露端点
m.Use(r)

r.GET("/product/:id", func(ctx *gin.Context) {
    ctx.JSON(200, gin.H{"productId": ctx.Param("id")})
})
_ = r.Run()
```

### 仅注册中间件（不暴露端点）

```go
m.UseWithoutExposingEndpoint(r)
// 在其他 router 上单独暴露
m.Expose(adminRouter)
```

## 自定义指标

### Gauge

```go
gaugeMetric := &ginmetrics.Metric{
    Type:        ginmetrics.Gauge,
    Name:        "example_gauge_metric",
    Description: "an example of gauge type metric",
    Labels:      []string{"label1"},
}
_ = ginmetrics.GetMonitor().AddMetric(gaugeMetric)

// 设置值
_ = ginmetrics.GetMonitor().GetMetric("example_gauge_metric").SetGaugeValue([]string{"label_value1"}, 0.1)
// 递增
_ = ginmetrics.GetMonitor().GetMetric("example_gauge_metric").Inc([]string{"label_value1"})
// 增加
_ = ginmetrics.GetMonitor().GetMetric("example_gauge_metric").Add([]string{"label_value1"}, 0.2)
```

### Counter

`Counter` 类型可以使用 `Inc` 和 `Add` 方法，但不能使用 `SetGaugeValue`。

### Histogram / Summary

使用 `Observe` 方法记录观测值。
