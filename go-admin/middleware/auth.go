package middleware

import (
	"go-admin/internal/repository"
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
			c.Abort()
			return
		}

		// 2. 截取 Bearer 后面的 token
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		// 3. 解析 JWT（验证签名 + 过期时间 + token_type）
		claims, err := utils.ParseAccessToken(tokenStr)
		if err != nil {
			if err.Error() == "token类型错误" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code": 0,
					"msg":  "token类型错误，请使用access_token",
				})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code": 0,
					"msg":  "token无效或登录已过期",
				})
			}
			c.Abort()
			return
		}

		// 4. 检查 jti 黑名单
		if blocked, _ := repository.CheckJTIBlacklist(claims.JTI); blocked {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "token已失效，请重新登录",
			})
			c.Abort()
			return
		}

		// 5. 把登录用户存入上下文
		c.Set("userId", claims.UserId)

		// 放行
		c.Next()
	}
}
