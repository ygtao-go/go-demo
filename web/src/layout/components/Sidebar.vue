<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { MagicStick, Odometer, User } from '@element-plus/icons-vue'

defineProps<{ collapsed: boolean }>()

const route = useRoute()

// 菜单配置（与 router/index.ts 子路由 meta 对应）
// TODO：后续从路由表自动生成 + 菜单权限控制
const menus = [
  { path: '/dashboard', title: '仪表盘', icon: Odometer },
  { path: '/user', title: '用户管理', icon: User },
  { path: '/ai', title: 'AI 助手', icon: MagicStick },
]

const activeMenu = computed(() => route.path)
</script>

<template>
  <div class="sidebar">
    <div class="sidebar-logo">
      <el-icon :size="28" color="#ffffff"><Odometer /></el-icon>
      <span v-show="!collapsed" class="sidebar-title">Go Admin</span>
    </div>

    <el-menu
      class="sidebar-menu"
      :default-active="activeMenu"
      :collapse="collapsed"
      :collapse-transition="false"
      router
      background-color="#001529"
      text-color="rgba(255, 255, 255, 0.65)"
      active-text-color="#ffffff"
    >
      <el-menu-item v-for="item in menus" :key="item.path" :index="item.path">
        <el-icon><component :is="item.icon" /></el-icon>
        <template #title>{{ item.title }}</template>
      </el-menu-item>
    </el-menu>
  </div>
</template>

<style scoped lang="scss">
.sidebar {
  height: 100%;
  background-color: #001529;

  .sidebar-logo {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    height: 56px;
    overflow: hidden;
    white-space: nowrap;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  }

  .sidebar-title {
    color: #ffffff;
    font-size: 16px;
    font-weight: 600;
  }

  .sidebar-menu {
    border-right: none;
  }
}
</style>
