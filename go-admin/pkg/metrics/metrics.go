// Package metrics 提供基于 Prometheus 的生产监控指标能力。
//
// 指标分类：
//  1. HTTP 层（由 Metrics() 中间件自动采集）：
//     - http_requests_total            请求总数（method / path / status）
//     - http_request_duration_seconds  请求耗时（method / path，直方图）
//     - http_errors_total              HTTP 错误数 status>=400（method / path / status）
//  2. 业务层（由业务代码手动上报）：
//     - refresh_success_total          refresh token 刷新成功次数
//     - refresh_failure_total          refresh token 刷新失败次数
//     - ai_calls_total                 AI 服务调用总次数
//     - ai_failures_total              AI 服务调用失败次数
//
// 使用方式：
//   - cmd/main.go 中挂载 r.Use(metrics.Metrics()) 采集 HTTP 指标
//   - router 中注册 r.GET("/metrics", metrics.Handler()) 暴露采集端点
//   - 业务代码中调用 metrics.RecordRefresh(success) / metrics.RecordAICall(success) 上报业务指标
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ==================== HTTP 指标 ====================

var (
	// HTTPRequestsTotal HTTP 请求总数，按 method / route / status 维度统计。
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP 请求总数，按 method / route / status 维度统计",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration HTTP 请求耗时（秒），按 method / route 维度统计。
	// 使用默认桶 [0.005s, 0.01s, 0.025s, ..., 10s]，可满足绝大多数接口延迟监控。
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP 请求耗时（秒），按 method / route 维度统计",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// HTTPErrorsTotal HTTP 错误数（status >= 400），按 method / route / status 维度统计。
	HTTPErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "HTTP 错误数（status >= 400），按 method / route / status 维度统计",
		},
		[]string{"method", "path", "status"},
	)
)

// ==================== 业务指标 ====================

var (
	// RefreshSuccessTotal refresh token 刷新成功次数。
	RefreshSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "refresh_success_total",
		Help: "refresh token 刷新成功次数",
	})
	// RefreshFailureTotal refresh token 刷新失败次数。
	RefreshFailureTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "refresh_failure_total",
		Help: "refresh token 刷新失败次数",
	})
	// AICallsTotal AI 服务调用总次数（成功 + 失败）。
	AICallsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ai_calls_total",
		Help: "AI 服务调用总次数",
	})
	// AIFailuresTotal AI 服务调用失败次数。
	AIFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ai_failures_total",
		Help: "AI 服务调用失败次数",
	})
)

// RecordRefresh 记录一次 refresh token 刷新结果。
// success=true 累加刷新成功次数，否则累加刷新失败次数。
func RecordRefresh(success bool) {
	if success {
		RefreshSuccessTotal.Inc()
	} else {
		RefreshFailureTotal.Inc()
	}
}

// RecordAICall 记录一次 AI 调用结果。
// 每次调用累加 AI 调用总次数；success=false 时同时累加 AI 失败次数。
func RecordAICall(success bool) {
	AICallsTotal.Inc()
	if !success {
		AIFailuresTotal.Inc()
	}
}
