package dto

// ==================== Dashboard 模块 DTO ====================

// DashboardStatistics Dashboard 统计数据
//
// 数据来源：
//   - UserCount    来自 MySQL users 表（SELECT COUNT(*) FROM users）
//   - 其余 4 项    来自 Redis 业务计数器（dashboard:ai_calls / dashboard:ai_errors /
//     dashboard:http_requests / dashboard:http_errors），key 不存在时按 0 处理
type DashboardStatistics struct {
	UserCount    int64 `json:"userCount"`    // 用户数量
	AICallCount  int64 `json:"aiCallCount"`  // AI 调用次数（成功）
	AIErrorCount int64 `json:"aiErrorCount"` // AI 调用失败次数
	RequestCount int64 `json:"requestCount"` // 接口请求次数
	ErrorCount   int64 `json:"errorCount"`   // 接口错误次数（status >= 400）
}

// DashboardResult Dashboard 统计响应：data 为 DashboardStatistics
type DashboardResult struct {
	Code int                 `json:"code"` // 业务码：0=成功
	Msg  string              `json:"msg"`  // 提示信息
	Data DashboardStatistics `json:"data"` // 统计数据
}
