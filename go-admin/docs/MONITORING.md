# go-admin 监控说明（MONITORING）

基于 **Prometheus 客户端库**（`prometheus/client_golang`）实现指标暴露，配合 Prometheus 采集与 Grafana 可视化。指标定义见 `pkg/metrics/metrics.go`，HTTP 采集中间件见 `pkg/metrics/http.go`。

## 1. 指标端点

- 路径：`GET /metrics`
- 格式：Prometheus 文本格式（`promhttp.Handler()`）
- 鉴权：**无需 JWT**（便于采集器直接抓取）
- 自排除：`/metrics` 自身请求不参与 HTTP 指标统计，避免采集动作干扰数据（`metricsPath` 常量）

## 2. 指标分类总览

指标分两类来源：

| 来源 | 方式 | 指标 |
|------|------|------|
| HTTP 层 | `Metrics()` 中间件自动采集（挂在 `cmd/main.go` 全局中间件链，且注册在 Recovery 之前，panic 恢复为 500 后仍统计） | `http_requests_total` / `http_request_duration_seconds` / `http_errors_total` |
| 业务层 | 业务代码手动上报 | `refresh_success_total` / `refresh_failure_total` / `ai_calls_total` / `ai_failures_total` |

## 3. HTTP 指标

### 3.1 `http_requests_total`（Counter）

- 维度：`method`、`path`、`status`
- 语义：HTTP 请求总数。
- 说明：`path` 优先使用路由模式（`c.FullPath()`，如 `/api/user/:id`）以保持低基数、方便按接口聚合；未命中路由（404）时退回真实 URL。

### 3.2 `http_request_duration_seconds`（Histogram）

- 维度：`method`、`path`
- 语义：请求耗时（秒），默认桶 `[0.005, 0.01, 0.025, ..., 10]`（`prometheus.DefBuckets`）。
- 使用：可用于计算 P50 / P95 / P99 延迟。

### 3.3 `http_errors_total`（Counter）

- 维度：`method`、`path`、`status`
- 语义：HTTP 错误数，仅 `status >= 400` 时累加。

## 4. 业务指标

| 指标 | 类型 | 语义 | 上报位置 |
|------|------|------|----------|
| `refresh_success_total` | Counter | Refresh Token 刷新成功次数 | `service.RefreshToken` 经 `defer metrics.RecordRefresh(err == nil)` |
| `refresh_failure_total` | Counter | Refresh Token 刷新失败次数 | 同上（`err != nil`） |
| `ai_calls_total` | Counter | AI 服务调用总次数（成功 + 失败） | `repository.CallLLM` 经 `defer metrics.RecordAICall(err == nil)` |
| `ai_failures_total` | Counter | AI 服务调用失败次数 | 同上（`err != nil`） |

上报函数：

```go
// pkg/metrics/metrics.go
func RecordRefresh(success bool)  // 成功→refresh_success_total+1，失败→refresh_failure_total+1
func RecordAICall(success bool)   // 每次 ai_calls_total+1；失败额外 ai_failures_total+1
```

## 5. 采集方式

### 5.1 Prometheus 抓取配置

```yaml
scrape_configs:
  - job_name: go-admin
    metrics_path: /metrics
    scrape_interval: 15s
    static_configs:
      - targets: ["<host>:8080"]
```

### 5.2 Docker 部署

`go-admin` 容器仅暴露 `8080` 端口，Prometheus 需与容器同网络或通过宿主机端口抓取 `http://localhost:8080/metrics`。

### 5.3 本地验证

```bash
curl -s http://localhost:8080/metrics | grep -E 'http_requests_total|ai_calls_total|refresh_success_total'
```

## 6. PromQL 查询示例

```promql
# 各接口请求 QPS（5 分钟平均）
sum(rate(http_requests_total[5m])) by (path)

# 接口错误率（status >= 400）
sum(rate(http_errors_total[5m])) by (path) / sum(rate(http_requests_total[5m])) by (path)

# P99 延迟（http_request_duration_seconds 直方图）
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))

# AI 调用成功率
sum(ai_calls_total) - sum(ai_failures_total)
sum(ai_calls_total)

# Refresh 失败占比
sum(refresh_failure_total) / (sum(refresh_success_total) + sum(refresh_failure_total))
```

## 7. 推荐告警规则（示例）

```yaml
groups:
  - name: go-admin
    rules:
      - alert: HighErrorRate
        expr: sum(rate(http_errors_total[5m])) / sum(rate(http_requests_total[5m])) > 0.05
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "go-admin 错误率超过 5%" }

      - alert: AIFailureHigh
        expr: sum(ai_failures_total) / sum(ai_calls_total) > 0.2
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "AI 调用失败率超过 20%" }
```

## 8. 指标测试

`pkg/metrics/metrics_test.go` 覆盖：

- `Metrics()` 中间件正确记录请求总数 / 错误数；
- `/metrics` 端点返回 Prometheus 文本格式且自身不计入请求总数；
- `RecordRefresh` / `RecordAICall` 上报逻辑（`prometheus/testutil` 断言）。

---

## 参考

- 指标与上报代码：`pkg/metrics/`
- 中间件挂载顺序：`cmd/main.go`
- 架构说明：[ARCHITECTURE.md](ARCHITECTURE.md)
