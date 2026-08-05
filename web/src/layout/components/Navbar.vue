<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Expand, Fold, SwitchButton } from '@element-plus/icons-vue'

import { useUserStore } from '@/stores/modules/user'

const { collapsed } = defineProps<{ collapsed: boolean }>()
const emit = defineEmits<{ 'toggle-sidebar': [] }>()

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const pageTitle = computed(() => (route.meta.title as string) || '')
const username = computed(() => userStore.username || '管理员')

// 进入受保护页面时拉取当前用户信息（token 失效由 request.ts 的 401 拦截兜底）
onMounted(() => {
  if (!userStore.userInfo) {
    userStore.getUserInfo().catch(() => {})
  }
})

async function handleLogout() {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      confirmButtonText: '退出登录',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }

  await userStore.logout()
  ElMessage.success('已退出登录')
  router.push('/login')
}
</script>

<template>
  <div class="navbar">
    <div class="navbar-left">
      <el-icon class="navbar-collapse" :size="20" @click="emit('toggle-sidebar')">
        <Expand v-if="collapsed" />
        <Fold v-else />
      </el-icon>
      <el-breadcrumb separator="/">
        <el-breadcrumb-item :to="{ path: '/dashboard' }">首页</el-breadcrumb-item>
        <el-breadcrumb-item v-if="pageTitle">{{ pageTitle }}</el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <div class="navbar-right">
      <el-dropdown>
        <span class="navbar-user">
          <el-avatar :size="28" class="navbar-avatar">{{ username.charAt(0) }}</el-avatar>
          <span>{{ username }}</span>
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="handleLogout">
              <el-icon><SwitchButton /></el-icon>
              退出登录
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </div>
</template>

<style scoped lang="scss">
.navbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 100%;
  padding: 0 16px;

  .navbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .navbar-collapse {
    cursor: pointer;
  }

  .navbar-right {
    display: flex;
    align-items: center;
  }

  .navbar-user {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    color: #303133;
    outline: none;
  }

  .navbar-avatar {
    background-color: #2563eb;
    color: #ffffff;
  }
}
</style>
