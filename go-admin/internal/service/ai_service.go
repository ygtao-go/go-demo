package service

import (
	"context"

	"go-admin/internal/repository"
)

// ==================== AI 模块 ====================

// GenerateCode 生成代码
func GenerateCode(prompt string) (string, error) {
	return repository.CallLLM("生成代码：" + prompt)
}

// GenerateStream AI 生成代码（SSE 流式输出）
// 只负责业务转发：接收 handler 传入的 ctx 与回调，转发到 repository.CallLLMStream。
// ctx 用于客户端断开时取消上游请求；callback 每收到一段内容被调用一次。
func GenerateStream(ctx context.Context, prompt string, callback func(string) error) error {
	return repository.CallLLMStream(ctx, "生成代码："+prompt, callback)
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
