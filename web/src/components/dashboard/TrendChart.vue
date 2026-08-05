<script setup lang="ts">
/**
 * TrendChart 统计图表
 *
 * 数据策略（严格遵守「不允许使用假数据」）：
 *   1. 优先展示后端提供的时间序列趋势（trend 属性，折线图）—— 扩展能力，待后端新增趋势接口后自动生效；
 *   2. 当前后端仅返回累计总量：以 series（真实累计值）渲染柱状图；
 *   3. 两者均为空时显示空态提示。
 */
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'

import type { ChartSeriesItem, TrendPoint } from '@/types/dashboard'

const props = withDefaults(
  defineProps<{
    /** 图表标题 */
    title: string
    /** 当前累计总量（后端真实值，展示在标题右侧） */
    total?: number
    /** 柱状图数据（真实累计值：名称 → 数值） */
    series?: ChartSeriesItem[]
    /** 时间序列趋势（扩展预留；为空时使用柱状图 / 空态） */
    trend?: TrendPoint[]
  }>(),
  {
    total: 0,
    series: () => [],
    trend: () => [],
  },
)

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

/** 构建 ECharts 配置 */
function buildOption(): EChartsOption {
  // ===== 模式一：时间序列趋势折线图（扩展能力，后端提供趋势接口后自动生效） =====
  if (props.trend.length > 0) {
    return {
      tooltip: { trigger: 'axis' },
      grid: { left: 48, right: 24, top: 32, bottom: 32 },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: props.trend.map((p) => {
          const t = new Date(p.timestamp)
          return Number.isNaN(t.getTime())
            ? String(p.timestamp)
            : `${t.getMonth() + 1}/${t.getDate()} ${String(t.getHours()).padStart(2, '0')}:${String(t.getMinutes()).padStart(2, '0')}`
        }),
      },
      yAxis: { type: 'value', minInterval: 1 },
      series: [
        {
          name: props.title,
          type: 'line',
          smooth: true,
          data: props.trend.map((p) => p.value),
          itemStyle: { color: '#2563eb' },
          lineStyle: { color: '#2563eb', width: 2 },
          areaStyle: { color: 'rgba(37, 99, 235, 0.08)' },
        },
      ],
    }
  }

  // ===== 模式二：当前累计总量柱状图（真实数据） =====
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: 48, right: 24, top: 32, bottom: 32 },
    xAxis: { type: 'category', data: props.series.map((s) => s.name) },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        name: props.title,
        type: 'bar',
        data: props.series.map((s) => s.value),
        barMaxWidth: 48,
        itemStyle: { color: '#2563eb', borderRadius: [4, 4, 0, 0] },
      },
    ],
  }
}

/** 渲染 / 更新图表（无数据时仅展示空态，不初始化 ECharts 实例） */
function renderChart() {
  if (!chartRef.value) return
  const isEmpty = props.series.length === 0 && props.trend.length === 0
  if (isEmpty) {
    chart?.clear()
    return
  }
  if (!chart) {
    chart = echarts.init(chartRef.value)
  }
  chart.setOption(buildOption(), true)
}

function handleResize() {
  chart?.resize()
}

onMounted(async () => {
  await nextTick()
  renderChart()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chart?.dispose()
  chart = null
})

watch(
  () => [props.series, props.trend, props.title, props.total],
  () => {
    nextTick(renderChart)
  },
  { deep: true },
)
</script>

<template>
  <el-card class="trend-chart" shadow="hover">
    <template #header>
      <div class="trend-chart__header">
        <span class="trend-chart__title">{{ title }}</span>
        <span v-if="series.length > 0 || total > 0" class="trend-chart__total">
          累计总量 {{ total.toLocaleString('zh-CN') }}
        </span>
      </div>
    </template>
    <div class="trend-chart__canvas" :class="{ 'is-empty': series.length === 0 && trend.length === 0 }">
      <div ref="chartRef" class="trend-chart__echarts" />
      <el-empty v-if="series.length === 0 && trend.length === 0" description="暂无统计数据" :image-size="80" />
    </div>
  </el-card>
</template>

<style scoped lang="scss">
.trend-chart {
  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  &__title {
    font-size: 15px;
    font-weight: 600;
    color: #303133;
  }

  &__total {
    font-size: 12px;
    color: #909399;
    background: #f4f4f5;
    border-radius: 10px;
    padding: 2px 10px;
  }

  &__canvas {
    position: relative;
    height: 300px;
  }

  &__echarts {
    width: 100%;
    height: 100%;
  }
}
</style>
