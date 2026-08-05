import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

import { getAccessToken } from '@/utils/auth'

/**
 * 路由表
 * - /login 为公开页（不进入主布局，meta.public = true）
 * - / 下为主布局（Layout），子路由为各业务模块
 * - meta.title 用于菜单 / 面包屑；meta.icon 为 Element Plus 图标名
 */
const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { title: '登录', public: true },
  },
  {
    path: '/',
    component: () => import('@/layout/index.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { title: '仪表盘', icon: 'Odometer' },
      },
      {
        path: 'user',
        name: 'User',
        component: () => import('@/views/user/index.vue'),
        meta: { title: '用户管理', icon: 'User' },
      },
      {
        path: 'ai',
        name: 'Ai',
        component: () => import('@/views/ai/index.vue'),
        meta: { title: 'AI 助手', icon: 'MagicStick' },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/error/404.vue'),
    meta: { title: '404', public: true },
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

const APP_TITLE = import.meta.env.VITE_APP_TITLE || 'Go Admin'

/**
 * 全局前置守卫：
 *   - 动态标题：document.title = `${meta.title} - ${APP_TITLE}`
 *   - 已登录访问 /login → 重定向 /dashboard
 *   - 未登录访问受保护页面（无 meta.public）→ 重定向 /login 并携带 redirect
 */
router.beforeEach((to) => {
  const token = getAccessToken()
  document.title = to.meta.title ? `${to.meta.title} - ${APP_TITLE}` : APP_TITLE

  // 已经登录：访问 /login 时跳转 /dashboard
  if (to.path === '/login' && token) {
    return { path: '/dashboard' }
  }

  // 未登录：禁止访问 dashboard / user / ai（meta.public = true 的页面除外）
  if (!to.meta.public && !token) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  return true
})

export default router
