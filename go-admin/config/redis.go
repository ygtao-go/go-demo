package config

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// RedisRequestTimeout 单次 Redis 请求的超时上限，防止无限等待
	RedisRequestTimeout = 3 * time.Second
)

// 全局 Redis 客户端 + 环境隔离前缀（整个项目统一使用）
var (
	// RedisClient 全局 Redis 客户端（goroutine-safe，可并发使用）
	RedisClient *redis.Client

	// RedisKeyPrefix 环境隔离前缀，来源于环境变量 REDIS_PREFIX（如 dev / prod），空表示不加前缀
	RedisKeyPrefix string
)

// getEnv 读取环境变量，为空时返回默认值
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvInt 读取环境变量并转为 int，非法或为空时返回默认值
func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("环境变量 %s=%q 无效，使用默认值 %d", key, v, def)
	}
	return def
}

// getEnvBool 读取环境变量并转为 bool（true/1/t/yes 视为 true）
func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			log.Printf("环境变量 %s=%q 无效，使用默认值 %v", key, v, def)
			return def
		}
		return b
	}
	return def
}

// RedisContext 返回带超时的 Redis 请求上下文。
// 所有 Redis 调用都必须通过它获取 context，避免使用无超时的 context.Background() 无限等待。
func RedisContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), RedisRequestTimeout)
}

// RedisKey 统一生成带环境前缀的 Redis key：<REDIS_PREFIX>:<part1>:<part2>:...
// 未配置 REDIS_PREFIX 时与原 key 完全一致，保证向后兼容。
func RedisKey(parts ...string) string {
	key := strings.Join(parts, ":")
	if RedisKeyPrefix == "" {
		return key
	}
	return RedisKeyPrefix + ":" + key
}

// InitRedis 初始化 Redis 连接（全局只调用一次）。
// 所有参数均来自环境变量，无硬编码；启动时执行 PING 检测，失败直接终止进程（fail-fast）。
func InitRedis() {
	// ===== 1. 连接地址（兼容旧变量 REDIS_ADDR） =====
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		host := getEnv("REDIS_HOST", "127.0.0.1")
		port := getEnv("REDIS_PORT", "6379")
		addr = net.JoinHostPort(host, port)
	}

	// ===== 2. 密码（兼容旧变量 REDIS_PASS） =====
	password := getEnv("REDIS_PASSWORD", "")
	if password == "" {
		password = os.Getenv("REDIS_PASS")
	}

	// ===== 2.5 运行环境与安全校验 =====
	// ENV：dev / prod（默认 dev，允许无密码连接）
	// 生产环境（ENV=prod）强制要求 Redis 密码非空，否则启动失败（fail-fast）
	envMode := getEnv("ENV", getEnv("APP_ENV", "dev"))
	if envMode == "prod" && password == "" {
		log.Fatalf("生产环境（ENV=prod）禁止 Redis 无密码连接，请设置 REDIS_PASSWORD 强密码")
	}
	log.Printf("Redis 运行环境: %s", envMode)

	// ===== 3. DB / 连接池 =====
	db := getEnvInt("REDIS_DB", 0)
	poolSize := getEnvInt("REDIS_POOL_SIZE", 20)
	minIdleConns := getEnvInt("REDIS_MIN_IDLE_CONNS", 5)

	// ===== 4. 环境隔离前缀 =====
	RedisKeyPrefix = strings.TrimSuffix(strings.TrimSpace(os.Getenv("REDIS_PREFIX")), ":")
	if RedisKeyPrefix != "" {
		log.Printf("Redis key 环境前缀: %s", RedisKeyPrefix)
	}

	options := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	// ===== 5. 可选 TLS（默认关闭，不强制） =====
	if getEnvBool("REDIS_TLS_ENABLE", false) {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		tlsCfg.InsecureSkipVerify = getEnvBool("REDIS_TLS_SKIP_VERIFY", false) // 仅开发环境建议开启
		options.TLSConfig = tlsCfg
		log.Println("Redis TLS 已启用")
	}

	RedisClient = redis.NewClient(options)

	// ===== 6. 启动 PING 检测：失败即终止启动（fail-fast） =====
	ctx, cancel := RedisContext()
	defer cancel()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis连接失败 addr=%s db=%d err=%v", addr, db, err)
	}
	log.Printf("Redis连接成功 ✅ addr=%s db=%d", addr, db)
}
