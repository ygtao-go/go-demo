package main

import (
	"go-admin/config"
	_ "go-admin/docs"
	"go-admin/middleware"
	"go-admin/pkg/metrics"
	"go-admin/router"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func init() {
	// 本地开发加载 .env
	// 服务器上不会加载 .env，而是用 docker -e 传入的环境变量
	if _, err := os.Stat(".env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			log.Println("加载 .env 文件失败")
		} else {
			log.Println("✅ 成功加载本地 .env 配置")
		}
	}
}

// @title go-admin API
// @version 1.0
// @description go-admin 后台管理系统 API（Go + Gin + GORM + Redis + JWT），内置 AI 代码助手模块。
// @description 统一响应信封：{"code": <int>, "msg": <string>, "data": <any|null>}；code=0 表示成功（AI 模块为 200）。
// @description 需鉴权接口请在 Authorize 中填写：Bearer <accessToken>。

// @contact.name go-admin
// @contact.url http://localhost:8080

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer 认证，填写格式：Bearer <accessToken>（登录/刷新接口返回）
func main() {

	// 1. 初始化MySQL
	config.InitDB()

	// 2. 初始化Redis
	config.InitRedis()

	r := gin.Default()

	// ========== 可观测性：请求 ID（必须最外层，供 Logger / Recovery / 链路追踪使用） ==========
	r.Use(middleware.RequestID())

	// ========== 可观测性：Prometheus HTTP 指标（注册在 Recovery 之前，panic 恢复为 500 后仍会统计） ==========
	r.Use(metrics.Metrics())

	r.Use(middleware.Logger())

	// ========== 只需要加这 1 行！限流中间件 ==========
	r.Use(middleware.RedisLimit())
	// ==============================================

	r.Use(middleware.Recovery()) // 你原来的异常捕获
	router.InitRouter(r)         // 路由

	log.Println("服务启动成功，监听端口:8080")
	r.Run(":8080")
}
