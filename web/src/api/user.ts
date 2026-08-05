/**
 * 用户管理相关 API（见 go-admin/docs/API.md 第 3 章，均需 JWT）
 *
 *   - GET    /user/info        获取当前用户信息
 *   - GET    /user             用户列表（服务端分页：page / pageSize）→ { list, total }
 *   - PUT    /user/:id         编辑用户（dto.EditUserReq：username / status 可选）
 *   - DELETE /user/:id         删除用户
 *   - PATCH  /user/:id/status  切换用户状态（dto.SwitchStatusReq：status 必填，1=正常 / 2=禁用）
 *
 * 注意：GET /user 仅支持 page / pageSize 查询参数，后端没有 keyword，禁止拼接 keyword。
 *
 * TODO（后续阶段实现）：修改密码 PUT /user/password（dto.UpdatePasswordReq）
 */
import { request } from '@/utils/request'

import type { PageResult } from '@/types/api'
import type { EditUserRequest, SwitchStatusRequest, UserInfo, UserListItem, UserQuery } from '@/types/user'

/** 获取当前用户信息：GET /api/user/info */
export function getUserInfo(): Promise<UserInfo> {
  return request<UserInfo>({ url: '/user/info', method: 'get' })
}

/** 用户列表（服务端分页）：GET /api/user?page=&pageSize= → { list, total } */
export function getUserList(params: UserQuery): Promise<PageResult<UserListItem>> {
  return request<PageResult<UserListItem>>({ url: '/user', method: 'get', params })
}

/** 编辑用户：PUT /api/user/:id（请求体字段与 dto.EditUserReq 一致） */
export function editUser(id: number, data: EditUserRequest): Promise<string> {
  return request<string>({ url: `/user/${id}`, method: 'put', data })
}

/** 删除用户：DELETE /api/user/:id */
export function deleteUser(id: number): Promise<string> {
  return request<string>({ url: `/user/${id}`, method: 'delete' })
}

/** 切换用户状态：PATCH /api/user/:id/status（status：1=正常 / 2=禁用） */
export function switchUserStatus(id: number, data: SwitchStatusRequest): Promise<string> {
  return request<string>({ url: `/user/${id}/status`, method: 'patch', data })
}

