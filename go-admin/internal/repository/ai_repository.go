package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"go-admin/config"
	"go-admin/pkg/metrics"
)

// ==================== AI 模块 ====================
// 本文件是 AI Provider 调用的唯一入口，repository 完全负责对第三方 LLM 的 HTTP 调用：
//   - HTTP Request 创建（context.WithTimeout）
//   - Header 设置
//   - JSON Marshal / Unmarshal
//   - HTTP Client（含 Timeout）
//   - Response 解析
//   - 错误处理（HTTP 状态码 / JSON 解析 / API 错误）
//
// API Key / Endpoint / URL / Timeout 全部从 config 层（环境变量）获取，无硬编码。
// 禁止 handler / service / utils 直接发起 AI HTTP 请求。

// aiMessage 对话消息结构
type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// aiRequest Chat Completions 请求体
type aiRequest struct {
	Model       string      `json:"model"`
	Messages    []aiMessage `json:"messages"`
	Temperature float64     `json:"temperature"`
}

// aiResponse Chat Completions 响应体
type aiResponse struct {
	Choices []struct {
		Message aiMessage `json:"message"`
	} `json:"choices"`
}

const (
	// aiSystemPrompt 系统提示词（代码学习助手）
	aiSystemPrompt = "你是专业代码学习助手，擅长代码生成、解释、修复与优化，输出简洁、可直接运行的内容，必要时可使用联网插件获取最新信息。"
	// aiTemperature 温度参数：调低温度，让代码输出更稳定
	aiTemperature = 0.3
)

// CallLLM 调用豆包 AI（支持联网插件，适配代码学习场景），返回生成的文本。
// 通过命名返回值 + defer 统一上报 AI 调用次数/失败次数业务指标，不改动任何返回语义。
func CallLLM(prompt string) (result string, err error) {
	// 业务指标：AI 调用次数 / 失败次数
	defer func() {
		metrics.RecordAICall(err == nil)
	}()

	// 从 config 层获取 AI 配置（环境变量读取）
	apiKey := config.AIAPIKey()
	endpoint := config.AIEndpoint()
	apiURL := config.AIURL()
	timeout := config.AITimeout()

	// 1. 构建请求体（system 提示词 + 用户 prompt）
	payload := aiRequest{
		Model: endpoint,
		Messages: []aiMessage{
			{Role: "system", Content: aiSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: aiTemperature,
	}

	// 2. JSON 序列化（错误处理）
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("AI 请求序列化失败: %w", err)
	}

	// 3. 创建带超时的上下文（context.WithTimeout）
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 4. 创建 HTTP Request（带 context，超时后自动取消）
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建 AI 请求失败: %w", err)
	}

	// 5. 设置 Header
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 6. HTTP Client（双保险超时：context.WithTimeout + client.Timeout）
	client := &http.Client{Timeout: timeout}

	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "", errors.New("AI 服务响应超时，请稍后重试")
		}
		return "", fmt.Errorf("AI 服务请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 7. HTTP 状态码检查（非 2xx 视为 API 错误）
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI 服务返回异常状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 8. 读取响应体（错误处理）
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 AI 响应失败: %w", err)
	}

	// 9. JSON 解析（错误处理）
	var aiResp aiResponse
	if err := json.Unmarshal(body, &aiResp); err != nil {
		return "", fmt.Errorf("解析 AI 响应失败: %w", err)
	}

	// 10. 提取结果；无 choices 视为 API 错误
	if len(aiResp.Choices) > 0 {
		return aiResp.Choices[0].Message.Content, nil
	}
	return "", errors.New("AI 服务未返回有效结果，请稍后再试")
}
