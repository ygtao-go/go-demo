<script setup lang="ts">
/**
 * StatCard 数据卡片
 *
 * 企业后台风格：左侧彩色图标 + 右侧标题与统计数值。
 * 所有数值来自后端真实统计接口（见 @/types/dashboard 数据来源说明）。
 */
import { computed } from 'vue'
import type { Component } from 'vue'
import { DataLine } from '@element-plus/icons-vue'

const props = withDefaults(
  defineProps<{
    /** 卡片标题 */
    title: string
    /** 统计数值（真实数据） */
    value: number
    /** 图标组件（Element Plus 图标） */
    icon?: Component
    /** 主题色（图标 / 数值高亮） */
    color?: string
    /** 数值后缀（如「次」「人」） */
    suffix?: string
    /** 卡片加载态 */
    loading?: boolean
  }>(),
  {
    icon: DataLine,
    color: '#409eff',
    suffix: '',
    loading: false,
  },
)

/** 千分位格式化；非数字时显示占位符 */
const displayValue = computed(() => {
  const v = props.value
  if (v === null || v === undefined || Number.isNaN(v)) return '--'
  return v.toLocaleString('zh-CN')
})
</script>

<template>
  <el-card class="stat-card" shadow="hover" v-loading="loading">
    <div class="stat-card__body">
      <div class="stat-card__icon" :style="{ backgroundColor: `${color}1a`, color }">
        <el-icon :size="26"><component :is="icon" /></el-icon>
      </div>
      <div class="stat-card__info">
        <div class="stat-card__title">{{ title }}</div>
        <div class="stat-card__value" :style="{ color }">
          {{ displayValue }}<span v-if="suffix" class="stat-card__suffix">{{ suffix }}</span>
        </div>
      </div>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
.stat-card {
  &__body {
    display: flex;
    align-items: center;
    gap: 14px;
  }

  &__icon {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 52px;
    height: 52px;
    border-radius: 12px;
  }

  &__info {
    min-width: 0;
  }

  &__title {
    font-size: 13px;
    color: #909399;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  &__value {
    margin-top: 4px;
    font-size: 24px;
    font-weight: 600;
    line-height: 1.2;
    font-variant-numeric: tabular-nums;
  }

  &__suffix {
    margin-left: 2px;
    font-size: 13px;
    font-weight: 400;
    color: #909399;
  }
}
</style>
