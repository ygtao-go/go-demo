package main

import (
	"go-admin/config"
	"go-admin/middleware"
	"go-admin/router"
	"log"

	"github.com/gin-gonic/gin"
)

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
