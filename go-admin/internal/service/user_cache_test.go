package service

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-admin/config"
	"go-admin/internal/repository"
	"go-admin/model"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

// TestMain 加载项目根目录 .env（测试工作目录为 internal/service，向上两级）
func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env")
	os.Exit(m.Run())
}

var (
	testSetupOnce sync.Once
	testSetupErr  error

	// testDBQueries 通过 GORM 回调统计 MySQL SELECT 次数
	testDBQueries int64
)

// setupTest 初始化 MySQL / Redis 并挂载查询计数器。
// 基础设施不可用时跳过测试（不 fail）。
func setupTest(t *testing.T) {
	t.Helper()
	testSetupOnce.Do(func() {
		testSetupErr = initTestEnv()
	})
	if testSetupErr != nil {
		t.Skipf("跳过集成测试（MySQL/Redis 不可用）: %v", testSetupErr)
	}
	atomic.StoreInt64(&testDBQueries, 0)
}

func initTestEnv() error {
	// 端口预检，避免 InitRedis 的 log.Fatalf 直接终止测试进程
	checks := []struct{ host, port, name string }{
		{os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), "MySQL"},
		{os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"), "Redis"},
	}
	for _, c := range checks {
		if c.host == "" || c.port == "" {
			return fmt.Errorf("缺少 %s 连接配置（请检查 .env）", c.name)
		}
		addr := net.JoinHostPort(c.host, c.port)
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			return fmt.Errorf("%s 不可达 %s: %w", c.name, addr, err)
		}
		conn.Close()
	}

	config.InitDB()
	config.InitRedis()

	// 注册 MySQL 查询计数器（统计所有 SELECT）
	return config.DB.Callback().Query().Before("gorm:query").Register("test:count-queries", func(db *gorm.DB) {
		atomic.AddInt64(&testDBQueries, 1)
	})
}

// TestCachePenetrationNoneCache 缓存穿透保护：
// 100 次请求随机不存在的用户名（10 个唯一用户名 × 10 次），
// 有 none 缓存后每个唯一用户名只产生 1 次 MySQL 查询（总计 10 次），而不是 100 次。
func TestCachePenetrationNoneCache(t *testing.T) {
	setupTest(t)

	const (
		uniqueUsers = 10
		requests    = 100
	)

	names := make([]string, uniqueUsers)
	for i := 0; i < uniqueUsers; i++ {
		names[i] = fmt.Sprintf("none_%d_%d", time.Now().UnixNano(), i)
	}
	t.Cleanup(func() {
		for _, n := range names {
			_ = repository.DeleteNoneCachedUser(n)
		}
	})

	start := atomic.LoadInt64(&testDBQueries)
	notFound := 0
	for i := 0; i < requests; i++ {
		_, err := GetUserByUsernameProtected(names[i%uniqueUsers])
		if errors.Is(err, ErrUserNotFound) {
			notFound++
		}
	}
	dbQueries := atomic.LoadInt64(&testDBQueries) - start

	t.Logf("随机不存在用户 %d 个 × 请求 %d 次 => 返回不存在 %d 次，实际 MySQL 查询 %d 次",
		uniqueUsers, requests, notFound, dbQueries)
	if notFound != requests {
		t.Errorf("期望 %d 次请求全部返回“用户不存在”，实际 %d 次", requests, notFound)
	}
	// 无保护：100 次请求 = 100 次 MySQL 查询；有 none 缓存：每唯一用户名仅 1 次
	if dbQueries > int64(uniqueUsers) {
		t.Errorf("缓存穿透保护失效：期望 MySQL 查询 <= %d 次，实际 %d 次", uniqueUsers, dbQueries)
	}
}

// TestCacheBreakdownSingleflight 缓存击穿保护：
// 删除用户缓存后，100 个并发请求同一用户名，singleflight 保证 MySQL 只查询 1 次。
func TestCacheBreakdownSingleflight(t *testing.T) {
	setupTest(t)

	username := fmt.Sprintf("sf_%d", time.Now().UnixNano())
	if err := Register(username, "test123456"); err != nil {
		t.Fatalf("准备测试用户失败: %v", err)
	}
	t.Cleanup(func() {
		if u, err := repository.GetUserByUsername(username); err == nil {
			_ = repository.DeleteUser(u.ID)
		}
		_ = repository.DeleteCachedUser(username)
		_ = repository.DeleteNoneCachedUser(username)
	})

	// 制造击穿场景：删除全部缓存，让 100 个并发请求同时穿透到 DB
	if err := repository.DeleteCachedUser(username); err != nil {
		t.Fatalf("删除用户缓存失败: %v", err)
	}
	_ = repository.DeleteNoneCachedUser(username)

	start := atomic.LoadInt64(&testDBQueries)

	const concurrency = 100
	var wg sync.WaitGroup
	errs := make([]error, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = GetUserByUsernameProtected(username)
		}(i)
	}
	wg.Wait()

	dbQueries := atomic.LoadInt64(&testDBQueries) - start
	success := 0
	for _, e := range errs {
		if e == nil {
			success++
		}
	}

	t.Logf("删除缓存后并发 %d 请求同一用户 => 成功 %d 次，实际 MySQL 查询 %d 次",
		concurrency, success, dbQueries)
	if success != concurrency {
		t.Errorf("期望 %d 个请求全部成功，实际 %d", concurrency, success)
	}
	if dbQueries != 1 {
		t.Errorf("缓存击穿保护失效：期望 MySQL 仅查询 1 次，实际 %d 次", dbQueries)
	}
}

// TestCacheTTLJitter 缓存雪崩保护：
// 写入 30 个 user:key，检查 Redis TTL 均落在 23h~25h 范围内，且存在随机差异（非固定 24h）。
func TestCacheTTLJitter(t *testing.T) {
	setupTest(t)

	const samples = 30
	usernames := make([]string, samples)
	for i := 0; i < samples; i++ {
		usernames[i] = fmt.Sprintf("ttl_%d_%d", time.Now().UnixNano(), i)
	}
	t.Cleanup(func() {
		for _, n := range usernames {
			_ = repository.DeleteCachedUser(n)
		}
	})

	ttls := make([]time.Duration, samples)
	for i, n := range usernames {
		user := &model.User{ID: uint(i + 1), Username: n, Status: 1}
		if err := repository.CacheUser(user, repository.UserCacheBaseTTL); err != nil {
			t.Fatalf("写入用户缓存失败: %v", err)
		}
		ttl, err := repository.GetCacheTTL(n)
		if err != nil {
			t.Fatalf("读取 TTL 失败: %v", err)
		}
		ttls[i] = ttl
	}

	minTTL, maxTTL := ttls[0], ttls[0]
	distinct := make(map[time.Duration]bool)
	for _, ttl := range ttls {
		distinct[ttl] = true
		if ttl < minTTL {
			minTTL = ttl
		}
		if ttl > maxTTL {
			maxTTL = ttl
		}
	}

	// Redis TTL 精度为秒，允许 ±2s 容差
	low := 23*time.Hour - 2*time.Second
	high := 25*time.Hour + 2*time.Second
	t.Logf("采样 %d 个 user:key => TTL 范围 [%v, %v]，不同 TTL 种数 %d",
		samples, minTTL, maxTTL, len(distinct))
	if minTTL < low || maxTTL > high {
		t.Errorf("TTL 超出 23h~25h 范围: min=%v max=%v", minTTL, maxTTL)
	}
	if len(distinct) < 2 {
		t.Errorf("TTL 疑似固定（无随机偏移，存在雪崩风险）: 不同 TTL 种数=%d", len(distinct))
	}
}
