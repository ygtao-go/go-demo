/**
 * 用户 Store（Pinia）
 *
 * state：
 *   - token          Access Token（初始值从 localStorage 恢复）
 *   - refreshToken   Refresh Token（初始值从 localStorage 恢复）
 *   - userInfo       当前用户信息（GET /user/info 获取）
 *
 * actions：
 *   - login()        调用 /user/login 并持久化 JWT 双 Token
 *   - logout()       调用 /user/logout 并清理本地登录态
 *   - getUserInfo()  调用 /user/info 获取当前用户信息
 */
import { defineStore } from 'pinia'

import { login as loginApi, logout as logoutApi } from '@/api/auth'
import { getUserInfo as getUserInfoApi } from '@/api/user'
import { clearTokens, getAccessToken, getRefreshToken, setTokens } from '@/utils/auth'
import type { LoginParams } from '@/types/auth'
import type { UserInfo } from '@/types/user'

interface UserState {
  token: string
  refreshToken: string
  userInfo: UserInfo | null
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    token: getAccessToken(),
    refreshToken: getRefreshToken(),
    userInfo: null,
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    username: (state) => state.userInfo?.username ?? '',
  },

  actions: {
    /** 登录：POST /api/user/login，成功后写入双 Token */
    async login(params: LoginParams) {
      const data = await loginApi(params)
      setTokens(data)
      this.token = data.accessToken
      this.refreshToken = data.refreshToken
      return data
    },

    /** 获取当前用户信息：GET /api/user/info */
    async getUserInfo() {
      const data = await getUserInfoApi()
      this.userInfo = data
      return data
    },

    /** 退出登录：POST /api/user/logout，本地登录态兜底清理 */
    async logout() {
      if (this.refreshToken) {
        try {
          await logoutApi({ refreshToken: this.refreshToken })
        } catch {
          // 登出接口失败（如 token 已失效）时忽略，保证本地清理兜底
        }
      }
      clearTokens()
      this.token = ''
      this.refreshToken = ''
      this.userInfo = null
    },
  },
})
