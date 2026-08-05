/**
 * 后端统一响应信封：{ code, msg, data }
 * 见 go-admin/docs/API.md
 */
export interface ApiResponse<T = unknown> {
  /** 业务码：0 成功；AI 模块为 200；失败时与 HTTP 状态码一致（400/401/404/429/500） */
  code: number
  msg: string
  data: T
}

/** 分页请求参数 */
export interface PageQuery {
  page?: number
  pageSize?: number
}

/** 分页响应数据 */
export interface PageResult<T = unknown> {
  list: T[]
  total: number
}
