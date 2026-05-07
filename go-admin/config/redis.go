package config

import (
	"context"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
)

// 全局Redis客户端 + 上下文（整个项目统一用这一个）
var (
	RedisClient *redis.Client
	Ctx         = context.Background()
)

// InitRedis 初始化Redis连接（全局只调用一次）
func InitRedis() {
	// 从环境变量读取Redis配置
	addr := os.Getenv("REDIS_ADDR")     // 例如 172.19.39.114:6379
	password := os.Getenv("REDIS_PASS") // Redis密码，如果没密码留空

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0, // 默认用DB 0
	})

	// 测试连接是否成功
	_, err := RedisClient.Ping(Ctx).Result()
	if err != nil {
		log.Fatalf("Redis连接失败: %v", err) // 连接失败直接终止，避免后续报错
	}
	log.Println("Redis连接成功 ✅")
}
