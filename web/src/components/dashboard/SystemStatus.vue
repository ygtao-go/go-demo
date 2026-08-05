<script setup lang="ts">
/**
 * SystemStatus 系统状态
 *
 * 显示：
 *   - 当前登录用户（来自 Pinia userStore / GET /user/info）
 *   - 后端连接状态（基于 dashboard 统计接口的真实请求结果：
 *     成功 → 连接正常；失败 → 连接断开）
 *   - 数据同步时间（最近一次刷新统计接口的时间）
 */
import { computed } from 'vue'
import { Connection, User } from '@element-plus/icons-vue'

const props = defineProps<{
  /** 当前登录用户名 */
  username: string
  /** 后端连接状态（由统计接口真实请求结果决定） */
  backendOnline: boolean
  /** 最近一次数据同步时间 */
  lastSyncTime: string
}>()

const statusText = computed(() => (props.backendOnline ? '连接正常' : '连接断开'))
const statusType = computed(() => (props.backendOnline ? 'success' : 'danger'))
</script>

<template>
  <el-card class="system-status" shadow="hover">
    <template #header>
      <div class="system-status__header">
        <span class="system-status__title">系统状态</span>
      </div>
    </template>
    <el-descriptions :column="3" border>
      <el-descriptions-item label="当前登录用户">
        <div class="system-status__item">
          <el-icon><User /></el-icon>
          <span>{{ username || '未获取' }}</span>
        </div>
      </el-descriptions-item>
      <el-descriptions-item label="后端连接状态">
        <el-tag :type="statusType" size="small">
          <el-icon class="system-status__dot"><Connection /></el-icon>
          {{ statusText }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="数据同步时间">
        <span>{{ lastSyncTime }}</span>
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>

<style scoped lang="scss">
.system-status {
  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  &__title {
    font-size: 15px;
    font-weight: 600;
    color: #303133;
  }

  &__item {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  &__dot {
    margin-right: 2px;
  }
}
</style>
