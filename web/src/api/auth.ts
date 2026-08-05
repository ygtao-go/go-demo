/**
 * 认证相关 API（见 go-admin/docs/API.md 第 2 章）
 *
 * 公开接口（无需 JWT）：
 *   - POST /user/login    登录 → TokenData（双 Token + JTI）
 *   - POST /user/refresh  刷新 Token（Rotation 机制，旧 refresh token 原子消费）
 * 需鉴权接口：
 *   - POST /user/logout   退出登录（请求体携带 refreshToken，access token 走请求头）
 */
import { request } from '@/utils/request'

import type { LoginParams, LogoutParams, RefreshParams, TokenData } from '@/types/auth'

/** 用户登录：POST /api/user/login */
export function login(data: LoginParams): Promise<TokenData> {
  return request<TokenData>({ url: '/user/login', method: 'post', data })
}

/** 刷新 Token（Rotation 机制）：POST /api/user/refresh */
export function refreshToken(data: RefreshParams): Promise<TokenData> {
  return request<TokenData>({ url: '/user/refresh', method: 'post', data })
}

/** 退出登录：POST /api/user/logout */
export function logout(data: LogoutParams): Promise<string> {
  return request<string>({ url: '/user/logout', method: 'post', data })
}
