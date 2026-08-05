/**
 * 用户模块类型定义
 *
 * 与后端完全对齐（见 go-admin/model/user.go、go-admin/internal/dto/user.go、go-admin/docs/API.md）：
 *   - model.User  JSON 字段：id / username / status / created_at / updated_at（password 经 json:"-" 隐藏，永不返回）
 *   - model.User  没有 email 字段，前端也不定义 email
 */
export interface UserInfo {
  id: number
  username: string
  status: number // 1=正常 2=禁用
  created_at?: string
  updated_at?: string
}

/** 用户列表项（GET /api/user 列表中的每一项，字段同 model.User） */
export interface UserListItem {
  id: number
  username: string
  status: number // 1=正常 2=禁用
  created_at?: string
  updated_at?: string
}

/** 用户列表查询参数（dto.PageReq，服务端分页；后端无 keyword 参数） */
export interface UserQuery {
  page?: number
  pageSize?: number
}

/** 编辑用户请求体（dto.EditUserReq，username / status 均可选，空或 0 不更新） */
export interface EditUserRequest {
  username?: string
  status?: number
}

/** 切换用户状态请求体（dto.SwitchStatusReq，status 必填：1=正常 / 2=禁用） */
export interface SwitchStatusRequest {
  status: number
}

/** 修改密码请求体（dto.UpdatePasswordReq） */
export interface UpdatePasswordParams {
  oldPassword: string
  newPassword: string
}

