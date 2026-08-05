/**
 * Dashboard 数据看板相关 API（见 go-admin/docs/API.md 第 3.12 节，需 JWT）
 *
 *   - GET /dashboard/statistics  统计数据 → DashboardStatistics
 *       · userCount    用户总数（MySQL users 表 COUNT(*)）
 *       · aiCallCount  AI 调用成功次数（Redis: dashboard:ai_calls）
 *       · aiErrorCount AI 调用失败次数（Redis: dashboard:ai_errors）
 *       · requestCount HTTP 请求总数（Redis: dashboard:http_requests）
 *       · errorCount   HTTP 错误数，status>=400（Redis: dashboard:http_errors）
 */
import { request } from '@/utils/request'

import type { DashboardStatistics } from '@/types/dashboard'

/** 获取 Dashboard 统计数据：GET /api/dashboard/statistics */
export function getDashboardStatistics(): Promise<DashboardStatistics> {
  return request<DashboardStatistics>({ url: '/dashboard/statistics', method: 'get' })
}
