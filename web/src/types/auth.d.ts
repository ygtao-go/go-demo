/** 登录请求体 */
export interface LoginParams {
  username: string
  password: string
}

/** 登录 / 刷新 Token 响应数据（JWT 双 Token + JTI） */
export interface TokenData {
  accessToken: string
  refreshToken: string
  accessJTI: string
  refreshJTI: string
}

/** 刷新 Token 请求体 */
export interface RefreshParams {
  refreshToken: string
}

/** 退出登录请求体 */
export interface LogoutParams {
  refreshToken: string
}
