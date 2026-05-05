package middleware

import (
	"go-admin/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTAuth JWT鉴权中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取请求头 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "请先登录",
			})
			c.Abort() // 拦截请求，不再往下执行
			return
		}

		// 2. 截取 Bearer 后面的 token
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		// 3. 解析并校验token
		claims, err := utils.ParseToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "token无效或登录已过期",
			})
			c.Abort()
			return
		}

		// 4. 把登录用户存入上下文，后续接口可以直接拿
		c.Set("userId", claims.UserId)

		// 放行
		c.Next()
	}
}
