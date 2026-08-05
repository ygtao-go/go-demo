/**
 * Dashboard 模块类型定义
 *
 * 与后端完全对齐（见 go-admin/internal/dto/dashboard.go、go-admin/docs/API.md）：
 *   - GET /api/dashboard/statistics（需 JWT）→ DashboardStatistics
 *
 * 数据来源：
 *   - userCount     MySQL users 表（SELECT COUNT(*) FROM users）
 *   - aiCallCount   Redis: dashboard:ai_calls（AI 调用成功次数）
 *   - aiErrorCount  Redis: dashboard:ai_errors（AI 调用失败次数）
 *   - requestCount  Redis: dashboard:http_requests（HTTP 请求总数）
 *   - errorCount    Redis: dashboard:http_errors（HTTP 错误数，status >= 400）
 */
export interface DashboardStatistics {
  /** 用户数量 */
  userCount: number
  /** AI 调用次数（成功） */
  aiCallCount: number
  /** AI 调用失败次数 */
  aiErrorCount: number
  /** 接口请求次数 */
  requestCount: number
  /** 接口错误次数（status >= 400） */
  errorCount: number
}

/** 图表序列项：{ name, value }（当前为真实累计总量） */
export interface ChartSeriesItem {
  name: string
  value: number
}

/**
 * 趋势图数据点（时间序列）。
 * 后端当前仅返回累计总量，尚未提供时间维度接口；该类型为后续扩展预留：
 * 待新增趋势接口后，TrendChart 组件将自动切换到时间序列折线图，无需改动页面。
 */
export interface TrendPoint {
  /** 时间戳（毫秒）或日期字符串 */
  timestamp: string | number
  /** 该时间点的统计值 */
  value: number
}
