package dto

import "go-admin/model"

// ==================== 响应 DTO（仅用于 Swagger/OpenAPI 文档展示，不影响运行时行为） ====================
// 运行时统一响应信封仍为 pkg/response.Result{code, msg, data}，
// 此处按不同接口的 data 形状定义文档化响应类型，便于 swag 生成精确的 OpenAPI schema。

// StringResult 通用字符串消息响应（注册 / 退出 / 修改密码 / 删除 / 状态切换等）
type StringResult struct {
	Code int    `json:"code"` // 业务码：0=成功
	Msg  string `json:"msg"`  // 提示信息
	Data string `json:"data"` // 成功提示文本
}

// TokenData 登录 / 刷新 Token 返回的 data 内容
type TokenData struct {
	AccessToken  string `json:"accessToken"`  // 访问令牌（15 分钟有效）
	RefreshToken string `json:"refreshToken"` // 刷新令牌（7 天有效）
	AccessJTI    string `json:"accessJTI"`    // Access Token 的 JTI（撤销用）
	RefreshJTI   string `json:"refreshJTI"`   // Refresh Token 的 JTI（旋转用）
}

// TokenResult 登录 / 刷新 Token 响应：data 为双 Token 数据
type TokenResult struct {
	Code int       `json:"code"` // 业务码：0=成功
	Msg  string    `json:"msg"`  // 提示信息
	Data TokenData `json:"data"` // 双 Token 数据
}

// UserResult 获取用户信息响应：data 为用户详情（password 不会返回）
type UserResult struct {
	Code int        `json:"code"` // 业务码：0=成功
	Msg  string     `json:"msg"`  // 提示信息
	Data model.User `json:"data"` // 用户详情
}

// UserListData 用户列表响应中的 data 内容
type UserListData struct {
	List  []model.User `json:"list"`  // 用户列表
	Total int64        `json:"total"` // 总记录数
}

// UserListResult 用户列表（分页）响应
type UserListResult struct {
	Code int          `json:"code"` // 业务码：0=成功
	Msg  string       `json:"msg"`  // 提示信息
	Data UserListData `json:"data"` // 列表数据
}

// AIResult AI 模块响应（兼容 web 前端：业务码固定 200，data 为纯文本）
type AIResult struct {
	Code int    `json:"code"` // 业务码：200=成功
	Msg  string `json:"msg"`  // 提示信息
	Data string `json:"data"` // AI 返回文本（生成的代码 / 解释 / 修复 / 优化结果）
}
