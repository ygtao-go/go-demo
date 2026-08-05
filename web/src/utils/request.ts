/**
 * Axios 实例（统一请求封装）
 *
 * 与 go-admin 后端对接约定（见 go-admin/docs/API.md）：
 *   - Base URL：/api（开发环境由 vite.config.ts 代理到 http://localhost:8080）
 *   - 鉴权方式：Authorization: Bearer <accessToken>
 *   - 统一响应信封：{ code, msg, data }
 *     · 成功 code = 0（AI 模块为 200）
 *     · 失败 code 与 HTTP 状态码一致（400 / 401 / 404 / 429 / 500）
 *
 * 响应拦截器行为：
 *   - HTTP 2xx + code 0/200：放行（由 request<T> 解包 data）
 *   - HTTP 2xx + 其它 code：ElMessage 提示 msg 并 reject
 *   - HTTP 401：清空本地 Token 并跳转 /login
 */
import axios from 'axios'
import type { AxiosError, AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'

import { clearTokens, getAccessToken } from '@/utils/auth'
import type { ApiResponse } from '@/types/api'

const service: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 120000,
})

// 请求拦截器：自动携带 Authorization: Bearer <accessToken>
service.interceptors.request.use((config) => {
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：统一处理 { code, msg, data } 信封
service.interceptors.response.use(
  (response: AxiosResponse<ApiResponse>) => {
    const res = response.data
    // 业务成功：code = 0（AI 模块为 200）
    if (res.code === 0 || res.code === 200) {
      return response
    }
    // 业务失败：code 与 HTTP 状态码一致，统一提示并 reject
    const message = res.msg || '请求失败'
    ElMessage.error(message)
    return Promise.reject(new Error(message))
  },
  (error: AxiosError<ApiResponse>) => {
    const status = error.response?.status
    const msg = error.response?.data?.msg || error.message || '网络请求失败'

    // 401：token 无效 / 过期 / 已登出 → 清空本地登录态并回登录页
    if (status === 401) {
      clearTokens()
      if (window.location.pathname !== '/login') {
        ElMessage.error(msg)
        window.location.href = '/login'
      }
      return Promise.reject(error)
    }

    ElMessage.error(msg)
    return Promise.reject(error)
  },
)

/**
 * 泛型请求函数：返回业务 data（即响应信封中的 data 字段，类型为 T）
 *
 * 示例：
 *   request<TokenData>({ url: '/user/login', method: 'post', data: params })
 */
export function request<T = unknown>(config: AxiosRequestConfig): Promise<T> {
  return service.request<ApiResponse<T>>(config).then((response) => response.data.data)
}

export default service
