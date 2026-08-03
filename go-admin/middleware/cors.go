package middleware

import "github.com/gin-gonic/gin"

// Cors 跨域中间件（全项目唯一 CORS 实现，由 router 全局注册）。
// 完整处理跨域响应头与 OPTIONS 预检请求。
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
