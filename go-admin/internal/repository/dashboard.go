package repository

import (
	"go-admin/config"
	"go-admin/model"

	"github.com/go-redis/redis/v8"
)

// ==================== Dashboard 业务统计 ====================
//
// 数据来源：
//   - 用户数：MySQL users 表 COUNT(*)（CountUsers）
//   - 业务计数：Redis 计数器 dashboard:<metric>（IncrDashboardCounter / GetDashboardCounter）
//
// 计数写入方：
//   - AI 调用：repository.CallLLM 成功/失败时同步计数（保留 Prometheus 指标）
//   - HTTP 请求：middleware.Logger 中间件同步计数（保留 Prometheus 指标）
//
// key 统一经 config.RedisKey 生成，自动携带环境前缀（REDIS_PREFIX 为空时与裸 key 完全一致）。

// Dashboard 统计指标名（拼接后为 dashboard:<metric>）
const (
	DashboardMetricAICalls      = "ai_calls"      // AI 调用成功次数
	DashboardMetricAIErrors     = "ai_errors"     // AI 调用失败次数
	DashboardMetricHTTPRequests = "http_requests" // HTTP 请求总数
	DashboardMetricHTTPErrors   = "http_errors"   // HTTP 错误数（status >= 400）
)

// dashboardCounterKey 构造带环境前缀的 dashboard 计数器 key（如 dashboard:ai_calls）
func dashboardCounterKey(metric string) string {
	return config.RedisKey("dashboard", metric)
}

// IncrDashboardCounter 对指定 dashboard 计数器 +1（Redis 原子自增，不存在时自动初始化为 1）
func IncrDashboardCounter(metric string) error {
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.Incr(ctx, dashboardCounterKey(metric)).Err()
}

// GetDashboardCounter 读取指定 dashboard 计数器；key 不存在时返回 0（nil 错误）
func GetDashboardCounter(metric string) (int64, error) {
	ctx, cancel := config.RedisContext()
	defer cancel()
	val, err := config.RedisClient.Get(ctx, dashboardCounterKey(metric)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}

// DeleteDashboardCounter 删除指定 dashboard 计数器（key 不存在时返回 nil）
func DeleteDashboardCounter(metric string) error {
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.Del(ctx, dashboardCounterKey(metric)).Err()
}

// CountUsers 统计用户总数（SELECT COUNT(*) FROM users）
func CountUsers() (int64, error) {
	var total int64
	err := config.DB.Model(&model.User{}).Count(&total).Error
	return total, err
}
