package main

import (
	"go-admin/config"
	"go-admin/middleware"
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
func main() {

	// 1. 初始化MySQL
	config.InitDB()

	// 2. 初始化Redis
	config.InitRedis()

	r := gin.Default()

	r.Use(middleware.Logger())

	// ========== 只需要加这 1 行！限流中间件 ==========
	r.Use(middleware.RedisLimit())
	// ==============================================

	r.Use(middleware.Recovery()) // 你原来的异常捕获
	router.InitRouter(r)         // 路由

	log.Println("服务启动成功，监听端口:8080")
	r.Run(":8080")
}
