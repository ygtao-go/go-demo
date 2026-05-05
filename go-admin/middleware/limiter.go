package middleware

import (
	"fmt"
	"go-admin/pkg/response"
	"net/http"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gin-gonic/gin"
)

// 每分钟 60 次
var apiLimiter = tollbooth.NewLimiter(60, &limiter.ExpirableOptions{
	DefaultExpirationTTL: time.Minute,
})

// 限流中间件（升级版：用户 + 接口 + IP）
func Limiter() gin.HandlerFunc {
	return func(c *gin.Context) {

		// 1. 默认用 IP
		key := c.ClientIP()

		// 2. 如果登录了 → 用 userId（更精细）
		if userId, exists := c.Get("userId"); exists {
			key = fmt.Sprintf("user:%v", userId)
		}

		// 3. 拼接接口路径（接口级限流）
		key = key + ":" + c.FullPath()

		// 4. 限流判断
		if apiLimiter.LimitReached(key) {
			response.Fail(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}

		c.Next()
	}
}
