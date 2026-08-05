<script setup lang="ts">
import { ref } from 'vue'
import AppMain from './components/AppMain.vue'
import Navbar from './components/Navbar.vue'
import Sidebar from './components/Sidebar.vue'

// 侧边栏折叠状态（TODO：后续迁移到 stores/modules/app.ts）
const collapsed = ref(false)

function handleToggleSidebar() {
  collapsed.value = !collapsed.value
}
</script>

<template>
  <el-container class="app-layout">
    <el-aside :width="collapsed ? '64px' : '220px'" class="app-aside">
      <Sidebar :collapsed="collapsed" />
    </el-aside>

    <el-container class="app-body">
      <el-header class="app-header" height="56px">
        <Navbar :collapsed="collapsed" @toggle-sidebar="handleToggleSidebar" />
      </el-header>

      <el-main class="app-main">
        <AppMain />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped lang="scss">
.app-layout {
  height: 100vh;
}

.app-aside {
  background-color: var(--sidebar-bg);
  transition: width 0.2s ease;
  overflow-x: hidden;
}

.app-header {
  padding: 0;
  background-color: var(--header-bg);
  box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08);
  z-index: 10;
}

.app-main {
  padding: 16px;
  background-color: var(--page-bg);
  overflow: auto;
}
</style>
