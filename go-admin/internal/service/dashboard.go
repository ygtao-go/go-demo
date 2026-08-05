package service

import (
	"go-admin/internal/dto"
	"go-admin/internal/repository"
)

// ==================== Dashboard 模块 ====================

// GetDashboardStatistics 获取 Dashboard 统计数据：
//   - userCount    来自 MySQL users 表 COUNT(*)
//   - 其余 4 项    来自 Redis 业务计数器（dashboard:*），key 不存在时按 0 处理
//
// 本层只负责组合 repository 数据并返回 DTO，不处理任何 HTTP 逻辑。
func GetDashboardStatistics() (dto.DashboardStatistics, error) {
	var stats dto.DashboardStatistics
	var err error

	// 1. 用户总数（MySQL）
	if stats.UserCount, err = repository.CountUsers(); err != nil {
		return stats, err
	}

	// 2. Redis 业务计数器（不存在时返回 0）
	if stats.AICallCount, err = repository.GetDashboardCounter(repository.DashboardMetricAICalls); err != nil {
		return stats, err
	}
	if stats.AIErrorCount, err = repository.GetDashboardCounter(repository.DashboardMetricAIErrors); err != nil {
		return stats, err
	}
	if stats.RequestCount, err = repository.GetDashboardCounter(repository.DashboardMetricHTTPRequests); err != nil {
		return stats, err
	}
	if stats.ErrorCount, err = repository.GetDashboardCounter(repository.DashboardMetricHTTPErrors); err != nil {
		return stats, err
	}

	return stats, nil
}
