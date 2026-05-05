package middleware

import (
	"fmt"
	"go-admin/config"
	"go-admin/pkg/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 默认限流配置（可以后续改成读取配置文件）
const (
	LimitCount  = 60          // 次数
	LimitWindow = time.Minute // 时间窗口
)

// RedisLimit 分布式限流（用户 + IP + 接口）
func RedisLimit() gin.HandlerFunc {
	return func(c *gin.Context) {

		rdb := config.RedisClient
		ctx := c.Request.Context()

		// ========================
		// 1. 构建唯一 key
		// ========================

		// 默认：IP
		keyPrefix := c.ClientIP()

		// 登录用户 → 用 userId（更精细）
		if userId, exists := c.Get("userId"); exists {
			keyPrefix = fmt.Sprintf("user:%v", userId)
		}

		// 拼接接口路径（细粒度控制）
		key := fmt.Sprintf("limit:%s:%s", keyPrefix, c.FullPath())

		// ========================
		// 2. 原子计数
		// ========================

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "限流服务异常")
			c.Abort()
			return
		}

		// ========================
		// 3. 设置过期时间（只在第一次）
		// ========================

		if count == 1 {
			err := rdb.Expire(ctx, key, LimitWindow).Err()
			if err != nil {
				response.Fail(c, http.StatusInternalServerError, "限流服务异常")
				c.Abort()
				return
			}
		}

		// ========================
		// 4. 判断是否超限
		// ========================

		if count > LimitCount {
			response.Fail(
				c,
				http.StatusTooManyRequests,
				fmt.Sprintf("请求过于频繁，请稍后再试（%d/%d）", count, LimitCount),
			)
			c.Abort()
			return
		}

		// ========================
		// 5. 继续请求
		// ========================

		c.Next()
	}
}
