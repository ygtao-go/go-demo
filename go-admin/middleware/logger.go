package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"go-admin/internal/repository"

	"github.com/gin-gonic/gin"
)

// requestIDKey gin context 中保存 request_id 的 key
const requestIDKey = "request_id"

// GenerateRequestID 生成唯一请求 ID（16 字节随机数 → 32 位 hex）。
// crypto/rand 失败概率极低；失败时退回时间戳方案，保证请求链路不中断。
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// RequestID 请求 ID 中间件：
//   - 优先复用客户端传入的 X-Request-ID（便于跨服务链路追踪 / 日志串联）
//   - 否则自动生成唯一 ID
//   - 写入响应头 X-Request-ID，并存入 gin context（request_id）
//
// 必须注册在所有需要 request_id 的中间件（Logger / Recovery）之前，即全局最外层。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = GenerateRequestID()
		}
		c.Set(requestIDKey, rid)
		c.Writer.Header().Set("X-Request-ID", rid)
		c.Next()
	}
}

// GetRequestID 从 gin context 读取当前请求的 request_id；未设置时返回空串。
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Logger 全局请求日志
// 记录：request_id | 时间 | client_ip | method | path | status | latency
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()

		// 执行后续中间件/接口
		c.Next()

		// ========================
		// 业务统计同步：Dashboard Redis 计数器（保留 Prometheus 指标，额外写 Redis）
		//   - dashboard:http_requests +1（每个请求）
		//   - dashboard:http_errors   +1（status >= 400）
		// 与 pkg/metrics 保持一致：/metrics 自身不计入，避免监控轮询污染业务数据；
		// 计数失败仅记录日志，不影响请求主流程。
		// ========================
		status := c.Writer.Status()
		if c.Request.URL.Path != "/metrics" {
			if err := repository.IncrDashboardCounter(repository.DashboardMetricHTTPRequests); err != nil {
				log.Printf("Dashboard 请求计数写入失败 path=%s err=%v", c.Request.URL.Path, err)
			}
			if status >= http.StatusBadRequest {
				if err := repository.IncrDashboardCounter(repository.DashboardMetricHTTPErrors); err != nil {
					log.Printf("Dashboard 错误计数写入失败 path=%s status=%d err=%v", c.Request.URL.Path, status, err)
				}
			}
		}

		// 打印日志（含 request_id / status / latency / client_ip）
		fmt.Printf(
			"[%s] request_id=%s client_ip=%s method=%s path=%s status=%d latency=%v\n",
			time.Now().Format("2006-01-02 15:04:05"),
			GetRequestID(c),
			c.ClientIP(),
			c.Request.Method,
			c.Request.URL.Path,
			status,
			time.Since(start),
		)
	}
}
