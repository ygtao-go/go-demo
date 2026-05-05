package router

import (
	"go-admin/api"
	"go-admin/middleware"

	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) {
	// 全局中间件
	r.Use(Cors())

	// ===== 公共接口（无需 JWT） =====
	public := r.Group("/api")
	{
		public.POST("/user/register", api.Register)
		public.POST("/user/login", api.Login)
	}

	// ===== 私有接口（需要 JWT） =====
	auth := r.Group("/api")
	auth.Use(middleware.JWTAuth())
	{
		// 用户模块
		user := auth.Group("/user")
		{
			user.GET("/info", api.GetUserInfo)
			user.POST("/logout", api.Logout)
			user.PUT("/password", api.UpdatePassword)

			user.GET("", api.UserList)          // 列表
			user.PUT("/:id", api.EditUser)      // 更新
			user.DELETE("/:id", api.DeleteUser) // 删除
			user.PATCH("/:id/status", api.SwitchStatus)
		}

		// AI 模块
		ai := auth.Group("/ai")
		{
			ai.POST("/generate", api.GenerateCode)
			ai.POST("/explain", api.ExplainCode)
			ai.POST("/fix", api.FixCode)
			ai.POST("/optimize", api.OptimizeCode)
		}
	}
}

// 跨域中间件
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
