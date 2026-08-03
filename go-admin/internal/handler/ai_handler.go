package handler

import (
	"go-admin/internal/dto"
	"go-admin/internal/service"
	"go-admin/pkg/response"

	"github.com/gin-gonic/gin"
)

// 请求 DTO 统一定义在 internal/dto/，handler 不再定义请求结构体。

// ==================== 生成代码 ====================

// GenerateCode AI 生成代码
// @Summary AI 生成代码
// @Description 根据需求描述生成代码（业务码固定 200，兼容 web 前端判断）
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.GenerateCodeReq true "生成代码请求"
// @Success 200 {object} dto.AIResult "生成成功"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "AI 服务异常"
// @Router /ai/generate [post]
func GenerateCode(c *gin.Context) {
	var req dto.GenerateCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	res, err := service.GenerateCode(req.Prompt)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}

	response.Success200(c, res)
}

// ==================== 解释代码 ====================

// ExplainCode AI 解释代码
// @Summary AI 解释代码
// @Description 解释给定代码的含义与逻辑（业务码固定 200）
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CodeReq true "解释代码请求"
// @Success 200 {object} dto.AIResult "解释成功"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "AI 服务异常"
// @Router /ai/explain [post]
func ExplainCode(c *gin.Context) {
	var req dto.CodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	res, err := service.ExplainCode(req.Code)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}

	response.Success200(c, res)
}

// ==================== 修复代码 ====================

// FixCode AI 修复代码
// @Summary AI 修复代码
// @Description 修复给定代码中的错误（业务码固定 200）
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CodeReq true "修复代码请求"
// @Success 200 {object} dto.AIResult "修复成功"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "AI 服务异常"
// @Router /ai/fix [post]
func FixCode(c *gin.Context) {
	var req dto.CodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	res, err := service.FixCode(req.Code)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}

	response.Success200(c, res)
}

// ==================== 优化代码 ====================

// OptimizeCode AI 优化代码
// @Summary AI 优化代码
// @Description 优化给定代码，使其更简洁高效（业务码固定 200）
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CodeReq true "优化代码请求"
// @Success 200 {object} dto.AIResult "优化成功"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "AI 服务异常"
// @Router /ai/optimize [post]
func OptimizeCode(c *gin.Context) {
	var req dto.CodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	res, err := service.OptimizeCode(req.Code)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}

	response.Success200(c, res)
}
