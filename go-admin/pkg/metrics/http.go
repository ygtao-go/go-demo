package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsPath 指标采集端点路径。
// 该路径自身不参与 HTTP 指标统计，避免采集动作干扰指标数据。
const metricsPath = "/metrics"

// Metrics Gin 中间件：自动采集 HTTP 请求总数 / 耗时 / 错误数。
//
// 注意：必须注册在 Recovery 中间件之前（即外层），
// 这样 handler 内 panic 被 Recovery 恢复为 500 后，仍能在此处完成统计。
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// /metrics 自身不统计（防止自我采集干扰与高基数膨胀）
		if c.Request.URL.Path == metricsPath {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		method := c.Request.Method
		path := routeLabel(c)
		statusStr := strconv.Itoa(status)

		HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
		if status >= http.StatusBadRequest {
			HTTPErrorsTotal.WithLabelValues(method, path, statusStr).Inc()
		}
	}
}

// routeLabel 提取路由标签：
//   - 命中已注册路由时使用路由模式（如 /api/user/:id），保持低基数，方便按接口聚合；
//   - 未命中路由（如 404）时退回真实 URL 路径。
func routeLabel(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return c.Request.URL.Path
}

// Handler 返回 /metrics 采集端点处理器（Prometheus 文本格式，经 gin 封装）。
func Handler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
