/**
 * JWT Token 存取工具（localStorage 持久化）
 *
 * Key 规划：
 *   - access_token    Access Token（后端有效期 15 分钟）
 *   - refresh_token   Refresh Token（后端有效期 7 天，刷新时旋转作废旧 token）
 *   - access_jti / refresh_jti  Token 唯一 ID（登录响应返回，登出 / 刷新时使用）
 *
 * 与 go-admin 后端约定（见 go-admin/docs/API.md）：
 *   - 请求头携带：Authorization: Bearer <accessToken>
 *   - 刷新接口：POST /api/user/refresh（请求体携带 refreshToken）
 */
import type { TokenData } from '@/types/auth'

export const TOKEN_KEY = 'access_token'
export const REFRESH_TOKEN_KEY = 'refresh_token'
export const ACCESS_JTI_KEY = 'access_jti'
export const REFRESH_JTI_KEY = 'refresh_jti'

function readToken(key: string): string {
  return localStorage.getItem(key) || ''
}

function writeToken(key: string, value: string) {
  if (value) {
    localStorage.setItem(key, value)
  } else {
    localStorage.removeItem(key)
  }
}

export function setAccessToken(token: string) {
  writeToken(TOKEN_KEY, token)
}

export function getAccessToken(): string {
  return readToken(TOKEN_KEY)
}

export function removeAccessToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export function setRefreshToken(token: string) {
  writeToken(REFRESH_TOKEN_KEY, token)
}

export function getRefreshToken(): string {
  return readToken(REFRESH_TOKEN_KEY)
}

export function removeRefreshToken() {
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}

/** 登录 / 刷新成功后一次性写入全部 Token 信息 */
export function setTokens(data: TokenData) {
  setAccessToken(data.accessToken)
  setRefreshToken(data.refreshToken)
  writeToken(ACCESS_JTI_KEY, data.accessJTI)
  writeToken(REFRESH_JTI_KEY, data.refreshJTI)
}

/** 登出 / 401 时清空全部 Token 信息 */
export function clearTokens() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
  localStorage.removeItem(ACCESS_JTI_KEY)
  localStorage.removeItem(REFRESH_JTI_KEY)
}
