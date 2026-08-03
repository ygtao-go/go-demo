package handler

import (
	"go-admin/internal/dto"
	"go-admin/internal/service"
	"go-admin/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// 请求 DTO 统一定义在 internal/dto/，handler 不再定义请求结构体。

// ==================== 登录 ====================

// Login 用户登录
// @Summary 用户登录
// @Description 登录成功返回双 Token（accessToken 15 分钟有效，refreshToken 7 天有效）
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body dto.LoginReq true "登录请求"
// @Success 200 {object} dto.TokenResult "登录成功"
// @Failure 400 {object} response.Result "参数错误或用户名/密码错误"
// @Router /user/login [post]
func Login(c *gin.Context) {
	var req dto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	result, err := service.Login(req.Username, req.Password)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	response.Success(c, result)
}

// ==================== 注册 ====================

// Register 用户注册
// @Summary 用户注册
// @Description 注册新用户（用户名 2~20 位，密码 6~20 位），成功后清除该用户名的"不存在"缓存
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body dto.RegisterReq true "注册请求"
// @Success 200 {object} dto.StringResult "注册成功"
// @Failure 400 {object} response.Result "参数错误或用户名已存在"
// @Router /user/register [post]
func Register(c *gin.Context) {
	var req dto.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	if err := service.Register(req.Username, req.Password); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	response.Success(c, "注册成功")
}

// ==================== 获取用户信息 ====================

// GetUserInfo 获取当前登录用户信息
// @Summary 获取用户信息
// @Description 返回当前登录用户详情（password 字段不会出现在任何响应中）
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.UserResult "用户信息"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 404 {object} response.Result "用户不存在"
// @Router /user/info [get]
func GetUserInfo(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		response.Fail(c, 401, "未登录")
		return
	}

	user, err := service.GetUserInfo(userId.(uint))
	if err != nil {
		response.Fail(c, 404, err.Error())
		return
	}

	response.Success(c, user)
}

// ==================== 退出登录 ====================

// Logout 退出登录
// @Summary 退出登录
// @Description 撤销 accessToken（JTI 黑名单，TTL=剩余有效期）与 refreshToken（双向索引删除 + 7 天黑名单）；accessToken 从请求头获取
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.LogoutReq true "退出请求（refreshToken 必填）"
// @Success 200 {object} dto.StringResult "退出成功"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "退出失败"
// @Router /user/logout [post]
func Logout(c *gin.Context) {
	// 从 Header 获取 access_token
	accessToken := c.GetHeader("Authorization")
	if len(accessToken) > 7 {
		accessToken = accessToken[7:]
	}

	// 从请求体获取 refresh_token
	var req dto.LogoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	if err := service.Logout(accessToken, req.RefreshToken); err != nil {
		response.Fail(c, 500, "退出失败")
		return
	}

	response.Success(c, "退出成功")
}

// ==================== 刷新 Token ====================

// RefreshToken 刷新 Token（Rotation 机制）
// @Summary 刷新 Token
// @Description 使用 refreshToken 换取全新的双 Token；旧 refreshToken 原子消费作废，并发刷新同一 refreshToken 仅一个成功
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body dto.RefreshReq true "刷新请求"
// @Success 200 {object} dto.TokenResult "刷新成功（返回全新双 Token）"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "refresh token 无效、已过期或已被消费"
// @Router /user/refresh [post]
func RefreshToken(c *gin.Context) {
	var req dto.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	result, err := service.RefreshToken(req.RefreshToken)
	if err != nil {
		response.Fail(c, 401, err.Error())
		return
	}

	response.Success(c, result)
}

// ==================== 修改密码 ====================

// UpdatePassword 修改密码
// @Summary 修改密码
// @Description 校验旧密码后更新为新密码，成功后删除用户缓存（需登录）
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdatePasswordReq true "修改密码请求"
// @Success 200 {object} dto.StringResult "修改成功"
// @Failure 400 {object} response.Result "参数错误或旧密码错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Router /user/password [put]
func UpdatePassword(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		response.Fail(c, 401, "未登录")
		return
	}

	var req dto.UpdatePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	if err := service.UpdatePassword(userId.(uint), req.OldPassword, req.NewPassword); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	response.Success(c, "修改成功")
}

// ==================== 用户列表 ====================

// UserList 用户列表（分页）
// @Summary 用户列表
// @Description 分页查询用户列表（page ≤0 时按 1；pageSize ≤0 或 >100 时按 10）
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码（默认 1）" default(1)
// @Param pageSize query int false "每页条数（默认 10，最大 100）" default(10)
// @Success 200 {object} dto.UserListResult "用户列表"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "查询失败"
// @Router /user [get]
func UserList(c *gin.Context) {
	var req dto.PageReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	users, total, err := service.ListUsers(req.Page, req.PageSize)
	if err != nil {
		response.Fail(c, 500, "查询失败")
		return
	}

	response.Success(c, gin.H{
		"list":  users,
		"total": total,
	})
}

// ==================== 编辑用户 ====================

// EditUser 编辑用户
// @Summary 编辑用户
// @Description 更新用户用户名 / 状态（字段为空或 0 时不更新）；改名会同步刷新缓存
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param request body dto.EditUserReq true "编辑请求"
// @Success 200 {object} dto.StringResult "更新成功"
// @Failure 400 {object} response.Result "参数错误或用户不存在"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Router /user/{id} [put]
func EditUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	var req dto.EditUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	if err := service.EditUser(uint(id), req.Username, req.Status); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	response.Success(c, "更新成功")
}

// ==================== 删除用户 ====================

// DeleteUser 删除用户
// @Summary 删除用户
// @Description 删除用户数据 + 用户缓存 + 该用户全部 refresh token 索引（需登录）
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Success 200 {object} dto.StringResult "删除成功"
// @Failure 400 {object} response.Result "参数错误或用户不存在"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "删除失败"
// @Router /user/{id} [delete]
func DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	if err := service.DeleteUser(uint(id)); err != nil {
		response.Fail(c, 500, "删除失败")
		return
	}

	response.Success(c, "删除成功")
}

// ==================== 状态切换 ====================

// SwitchStatus 切换用户状态
// @Summary 切换用户状态
// @Description 切换用户状态：1=正常，2=禁用（其它值返回 400）
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param request body dto.SwitchStatusReq true "状态切换请求"
// @Success 200 {object} dto.StringResult "状态更新成功"
// @Failure 400 {object} response.Result "参数错误或状态值无效"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Router /user/{id}/status [patch]
func SwitchStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	var req dto.SwitchStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	if err := service.SwitchStatus(uint(id), req.Status); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	response.Success(c, "状态更新成功")
}
