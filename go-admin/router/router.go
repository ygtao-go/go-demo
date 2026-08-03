package router

import (
	"go-admin/internal/handler"
	"go-admin/middleware"
	"go-admin/pkg/metrics"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func InitRouter(r *gin.Engine) {
	// 全局中间件（CORS 统一使用 middleware.Cors）
	r.Use(middleware.Cors())

	// ===== 监控指标（Prometheus 采集端点，无需 JWT） =====
	r.GET("/metrics", metrics.Handler())

	// ===== Swagger / OpenAPI 接口文档（无需 JWT） =====
	// 访问：http://localhost:8080/swagger/index.html
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// ===== 公共接口（无需 JWT） =====
	public := r.Group("/api")
	{
		public.POST("/user/register", handler.Register)
		public.POST("/user/login", handler.Login)
		public.POST("/user/refresh", handler.RefreshToken)
	}

	// ===== 私有接口（需要 JWT） =====
	auth := r.Group("/api")
	auth.Use(middleware.JWTAuth())
	{
		// 用户模块
		user := auth.Group("/user")
		{
			user.GET("/info", handler.GetUserInfo)
			user.POST("/logout", handler.Logout)
			user.PUT("/password", handler.UpdatePassword)

			user.GET("", handler.UserList)          // 列表
			user.PUT("/:id", handler.EditUser)      // 更新
			user.DELETE("/:id", handler.DeleteUser) // 删除
			user.PATCH("/:id/status", handler.SwitchStatus)
		}

		// AI 模块（已迁移至 internal/handler）
		ai := auth.Group("/ai")
		{
			ai.POST("/generate", handler.GenerateCode)
			ai.POST("/explain", handler.ExplainCode)
			ai.POST("/fix", handler.FixCode)
			ai.POST("/optimize", handler.OptimizeCode)
		}
	}
}
