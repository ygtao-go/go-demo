<script setup lang="ts">
/**
 * 仪表盘（Dashboard 数据看板）
 *
 * 数据来源（全部真实，无假数据）：
 *   - GET /api/dashboard/statistics（需 JWT）
 *     · userCount    MySQL users 表 COUNT(*)
 *     · aiCallCount  Redis: dashboard:ai_calls
 *     · aiErrorCount Redis: dashboard:ai_errors
 *     · requestCount Redis: dashboard:http_requests
 *     · errorCount   Redis: dashboard:http_errors
 *
 * 页面结构：
 *   1. 5 张数据卡片（用户 / AI 调用 / AI 失败 / 接口请求 / 接口错误）
 *   2. 图表区（ECharts）：请求统计图、AI 调用统计图（当前展示真实累计总量柱状图，
 *      组件已预留时间序列趋势扩展能力）
 *   3. 系统状态：当前登录用户 + 后端连接状态 + 数据同步时间
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { CircleClose, DataAnalysis, MagicStick, Refresh, User, Warning } from '@element-plus/icons-vue'

import { getDashboardStatistics } from '@/api/dashboard'
import StatCard from '@/components/dashboard/StatCard.vue'
import SystemStatus from '@/components/dashboard/SystemStatus.vue'
import TrendChart from '@/components/dashboard/TrendChart.vue'
import { useUserStore } from '@/stores/modules/user'
import type { ChartSeriesItem, DashboardStatistics } from '@/types/dashboard'

const userStore = useUserStore()

// ==================== 状态 ====================

const loading = ref(false)
const refreshing = ref(false)
const backendOnline = ref(false)
const lastSyncTime = ref('--')

/** 统计结果（初始为全 0，接口返回后覆盖为真实数据） */
const stats = ref<DashboardStatistics>({
  userCount: 0,
  aiCallCount: 0,
  aiErrorCount: 0,
  requestCount: 0,
  errorCount: 0,
})

const username = computed(() => userStore.username || '管理员')

// ==================== 数据请求 ====================

/** 拉取统计接口：成功 → 后端在线 + 覆盖真实数据；失败 → 后端离线（保留上一次数据） */
async function fetchStatistics() {
  loading.value = true
  try {
    const data = await getDashboardStatistics()
    stats.value = data
    backendOnline.value = true
  } catch {
    backendOnline.value = false
  } finally {
    loading.value = false
    lastSyncTime.value = new Date().toLocaleString('zh-CN')
  }
}

/** 手动刷新（按钮 loading 态，避免与自动刷新混淆） */
async function handleRefresh() {
  refreshing.value = true
  try {
    await fetchStatistics()
  } finally {
    refreshing.value = false
  }
}

// ==================== 图表数据（真实累计值） ====================

/** 请求统计图：总请求数 vs 错误请求数 */
const requestChartSeries = computed<ChartSeriesItem[]>(() => [
  { name: '总请求数', value: stats.value.requestCount },
  { name: '错误请求数', value: stats.value.errorCount },
])

/** AI 调用统计图：AI 调用次数 vs AI 失败次数 */
const aiChartSeries = computed<ChartSeriesItem[]>(() => [
  { name: 'AI 调用次数', value: stats.value.aiCallCount },
  { name: 'AI 失败次数', value: stats.value.aiErrorCount },
])

// ==================== 生命周期 ====================

/** 自动刷新间隔（毫秒） */
const AUTO_REFRESH_INTERVAL = 30_000

let timer: number | undefined

onMounted(async () => {
  // 确保显示当前登录用户（Navbar 已兜底，此处再确认一次）
  if (!userStore.userInfo) {
    userStore.getUserInfo().catch(() => {})
  }
  await fetchStatistics()
  timer = window.setInterval(fetchStatistics, AUTO_REFRESH_INTERVAL)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<template>
  <div class="dashboard-page">
    <!-- 顶部：标题 + 刷新 -->
    <div class="dashboard-page__toolbar">
      <div>
        <h2 class="dashboard-page__title">数据看板</h2>
        <p class="dashboard-page__subtitle">系统运行指标一览（每 30 秒自动刷新）</p>
      </div>
      <el-button type="primary" :icon="Refresh" :loading="refreshing" @click="handleRefresh">刷新数据</el-button>
    </div>

    <!-- 1. 数据卡片 -->
    <el-row :gutter="16" class="dashboard-page__cards">
      <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
        <StatCard title="用户数量" :value="stats.userCount" :loading="loading" suffix="人" :icon="User" color="#2563eb" />
      </el-col>
      <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
        <StatCard title="AI 调用次数" :value="stats.aiCallCount" :loading="loading" suffix="次" :icon="MagicStick" color="#8b5cf6" />
      </el-col>
      <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
        <StatCard title="AI 失败次数" :value="stats.aiErrorCount" :loading="loading" suffix="次" :icon="Warning" color="#f59e0b" />
      </el-col>
      <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
        <StatCard title="接口请求次数" :value="stats.requestCount" :loading="loading" suffix="次" :icon="DataAnalysis" color="#10b981" />
      </el-col>
      <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
        <StatCard title="接口错误次数" :value="stats.errorCount" :loading="loading" suffix="次" :icon="CircleClose" color="#ef4444" />
      </el-col>
    </el-row>

    <!-- 2. 图表区域 -->
    <el-row :gutter="16" class="dashboard-page__charts">
      <el-col :xs="24" :lg="12">
        <TrendChart title="请求统计图" :total="stats.requestCount" :series="requestChartSeries" />
      </el-col>
      <el-col :xs="24" :lg="12">
        <TrendChart title="AI 调用统计图" :total="stats.aiCallCount" :series="aiChartSeries" />
      </el-col>
    </el-row>

    <!-- 3. 系统状态 -->
    <SystemStatus :username="username" :backend-online="backendOnline" :last-sync-time="lastSyncTime" />
  </div>
</template>

<style scoped lang="scss">
.dashboard-page {
  &__toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 16px;
  }

  &__title {
    margin: 0;
    font-size: 20px;
    font-weight: 600;
    color: #303133;
  }

  &__subtitle {
    margin: 4px 0 0;
    font-size: 13px;
    color: #909399;
  }

  &__cards {
    margin-bottom: 0;
  }

  &__cards :deep(.el-col) {
    margin-bottom: 16px;
  }

  &__charts {
    margin-bottom: 0;
  }

  &__charts :deep(.el-col) {
    margin-bottom: 16px;
  }
}
</style>

