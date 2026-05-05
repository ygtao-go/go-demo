package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 全局请求日志
// 打印：时间 | IP | 请求方式 | 路径 | 耗时
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()
		// 请求信息
		path := c.Request.URL.Path
		ip := c.ClientIP()
		method := c.Request.Method

		// 执行后续中间件/接口
		c.Next()

		// 打印日志
		fmt.Printf(
			"[%s] %15s | %6s | %s | %v\n",
			time.Now().Format("2006-01-02 15:04:05"),
			ip,
			method,
			path,
			time.Since(start),
		)
	}
}
