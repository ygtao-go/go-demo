package dto

// ==================== AI 模块请求 DTO ====================

// GenerateCodeReq 生成代码请求
type GenerateCodeReq struct {
	Prompt string `json:"prompt" binding:"required"`
}

// CodeReq 代码处理请求（解释 / 修复 / 优化共用）
type CodeReq struct {
	Code string `json:"code" binding:"required"`
}
