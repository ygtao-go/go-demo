package middleware

import (
	"fmt"
	"go-admin/pkg/response"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery 全局异常捕获中间件（工程级版本）
// 错误记录：request_id | error | method/path | client_ip | stack trace
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {

		defer func() {
			if err := recover(); err != nil {

				// ========================
				// 1. 打印错误日志（后端日志）
				// ========================
				fmt.Printf("\n===== PANIC RECOVER =====\n")
				fmt.Printf("request_id: %s\n", GetRequestID(c))
				fmt.Printf("error: %v\n", err)
				fmt.Printf("path: %s %s\n", c.Request.Method, c.Request.URL.Path)
				fmt.Printf("client_ip: %s\n", c.ClientIP())
				fmt.Printf("stack:\n%s\n", debug.Stack())
				fmt.Printf("=========================\n\n")

				// ========================
				// 2. 根据环境返回不同信息
				// ========================
				if gin.Mode() == gin.DebugMode {
					// 开发环境：返回详细错误（方便调试）
					response.Fail(c, http.StatusInternalServerError, fmt.Sprintf("panic: %v", err))
				} else {
					// 生产环境：隐藏错误细节（安全）
					response.Fail(c, http.StatusInternalServerError, "服务器内部错误")
				}

				// ========================
				// 3. 终止请求
				// ========================
				c.Abort()
			}
		}()

		c.Next()
	}
}
