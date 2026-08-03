package service

import "go-admin/internal/repository"

// ==================== AI 模块 ====================

// GenerateCode 生成代码
func GenerateCode(prompt string) (string, error) {
	return repository.CallLLM("生成代码：" + prompt)
}

// ExplainCode 解释代码
func ExplainCode(code string) (string, error) {
	return repository.CallLLM("解释这段代码：\n" + code)
}

// FixCode 修复代码
func FixCode(code string) (string, error) {
	return repository.CallLLM("修复这段代码的错误：\n" + code)
}

// OptimizeCode 优化代码
func OptimizeCode(code string) (string, error) {
	return repository.CallLLM("优化这段代码，让它更简洁高效：\n" + code)
}
