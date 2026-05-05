package config

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
)

// 全局Redis客户端 + 上下文（整个项目统一用这一个）
var (
	RedisClient *redis.Client
	Ctx         = context.Background()
)

// InitRedis 初始化Redis连接（全局只调用一次）
func InitRedis() {
	// 用你Linux服务器的IP和端口，完全保留你的配置
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379", // 你的Linux Redis地址，不用改
		Password: "",               // 无密码留空，有密码填这里
		DB:       0,                // 默认用DB 0
	})

	// 测试连接是否成功
	_, err := RedisClient.Ping(Ctx).Result()
	if err != nil {
		log.Fatalf("Redis连接失败: %v", err) // 连接失败直接终止，避免后续报错
	}
	log.Println("Redis连接成功 ✅")
}
