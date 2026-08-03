package tests

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"go-admin/config"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ============================================================
// 接口自动化测试：测试环境隔离方案
//
// 1. 数据库隔离：使用独立 schema go_admin_test（可通过 TEST_DB_NAME 覆盖），
//    与生产库 go_admin 完全隔离；每次运行前/后清空 users 表。
// 2. Redis 隔离：使用独立 key 前缀 test（可通过 TEST_REDIS_PREFIX 覆盖），
//    应用所有 key 统一经 config.RedisKey 加前缀；每次运行前/后扫描并删除 test:* 全部 key。
//
// 基础设施不可达时自动跳过（不 fail），保证无 MySQL/Redis 的环境仍可编译运行。
// ============================================================

const (
	// defaultTestDBName 测试专用数据库（与生产 go_admin 完全隔离）
	defaultTestDBName = "go_admin_test"
	// defaultTestRedisPrefix 测试 Redis key 前缀（与生产无前缀 / 其他前缀隔离）
	defaultTestRedisPrefix = "test"
)

func TestMain(m *testing.M) {
	// 1. 加载项目根目录 .env（测试工作目录为 tests，向上 1 级）
	_ = godotenv.Load("../.env")

	// 2. 基础设施可达性预检（失败则跳过整个测试包，不报错）
	if err := precheckInfra(); err != nil {
		fmt.Printf("SKIP 接口集成测试（MySQL/Redis 不可用）: %v\n", err)
		os.Exit(0)
	}

	// 3. 数据库隔离：确保独立测试 schema 存在
	if err := ensureTestDB(); err != nil {
		fmt.Printf("SKIP 接口集成测试（无法准备测试数据库）: %v\n", err)
		os.Exit(0)
	}

	// 4. 覆盖环境变量 → 连接测试库 + 测试 Redis 前缀（config 在调用时读取环境变量，覆盖生效）
	os.Setenv("DB_NAME", testDBName())
	os.Setenv("REDIS_PREFIX", testRedisPrefix())
	os.Setenv("ENV", "dev") // 测试环境固定 dev，避免 prod 强制 Redis 密码校验触发 Fatalf

	// 5. 初始化（预检已通过，此处不会 panic / Fatalf）
	config.InitDB()
	config.InitRedis()

	// 6. 清理历史残留数据，从干净状态开始
	cleanupTestData()

	code := m.Run()

	// 7. 结束后再次清理，保证不留任何测试数据
	cleanupTestData()
	os.Exit(code)
}

func testDBName() string {
	if v := os.Getenv("TEST_DB_NAME"); v != "" {
		return v
	}
	return defaultTestDBName
}

func testRedisPrefix() string {
	if v := os.Getenv("TEST_REDIS_PREFIX"); v != "" {
		return v
	}
	return defaultTestRedisPrefix
}

// precheckInfra 预检 MySQL / Redis TCP 可达性。
// 与 internal/service 缓存测试同一策略：InitRedis 内部为 log.Fatalf（直接终止进程），
// 必须先探测端口，避免测试进程被杀掉。
func precheckInfra() error {
	checks := []struct{ host, port, name string }{
		{os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), "MySQL"},
		{os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"), "Redis"},
	}
	for _, c := range checks {
		if c.host == "" || c.port == "" {
			return fmt.Errorf("缺少 %s 连接配置（请检查 .env）", c.name)
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(c.host, c.port), 2*time.Second)
		if err != nil {
			return fmt.Errorf("%s 不可达 %s:%s: %w", c.name, c.host, c.port, err)
		}
		conn.Close()
	}
	return nil
}

// ensureTestDB 连接 MySQL 服务端（不指定库），确保独立测试 schema 存在。
// 生产库 go_admin 完全不触碰。
func ensureTestDB() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接 MySQL 服务端失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return db.Exec("CREATE DATABASE IF NOT EXISTS " + testDBName()).Error
}

// cleanupTestData 清空测试库 users 表 + 删除测试前缀下全部 Redis key。
func cleanupTestData() {
	if config.DB != nil {
		if err := config.DB.Exec("DELETE FROM users").Error; err != nil {
			fmt.Printf("WARN 清空测试用户表失败: %v\n", err)
		}
	}
	if config.RedisClient != nil {
		if err := cleanupRedisKeys(); err != nil {
			fmt.Printf("WARN 清理测试 Redis key 失败: %v\n", err)
		}
	}
}

// cleanupRedisKeys 扫描并删除测试前缀（如 test:*）下的全部 key。
func cleanupRedisKeys() error {
	ctx, cancel := config.RedisContext()
	defer cancel()

	var cursor uint64
	pattern := testRedisPrefix() + ":*"
	for {
		keys, next, err := config.RedisClient.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := config.RedisClient.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
