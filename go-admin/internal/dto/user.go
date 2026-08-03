package dto

// ==================== 用户模块请求 DTO ====================

// LoginReq 登录请求
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterReq 注册请求
type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UpdatePasswordReq 修改密码请求
type UpdatePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// EditUserReq 编辑用户请求
type EditUserReq struct {
	Username string `json:"username"`
	Status   int    `json:"status"`
}

// SwitchStatusReq 状态切换请求
type SwitchStatusReq struct {
	Status int `json:"status" binding:"required"`
}

// PageReq 分页查询请求
type PageReq struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// RefreshReq 刷新 Token 请求
type RefreshReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// LogoutReq 退出登录请求
type LogoutReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}
