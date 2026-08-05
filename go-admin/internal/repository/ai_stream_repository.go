package repository

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go-admin/config"
	"go-admin/pkg/metrics"
)

// ==================== AI 模块：SSE 流式输出 ====================
// 本文件是 AI Provider 流式调用（SSE）的唯一入口，与 ai_repository.go 中 CallLLM 保持同层职责：
//   - context 传递（客户端断开时自动取消上游 HTTP 请求）
//   - Header 设置（Authorization / Content-Type / Accept）
//   - HTTP Client（保留 CallLLM 的 Transport 修复：显式 HTTP/1.1，不恢复默认 HTTP/2）
//   - SSE 帧逐行解析：data: {"choices":[{"delta":{"content":"xxx"}}]}
//   - 收到内容立即回调 callback(chunk)
//   - data: [DONE] 正常结束
//
// 与 CallLLM 一致：禁止 handler / service / utils 直接发起 AI HTTP 请求。

// aiStreamMessage SSE 流式请求对话消息
type aiStreamMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// aiStreamRequest SSE 流式 Chat Completions 请求体（stream: true）
type aiStreamRequest struct {
	Model       string            `json:"model"`
	Messages    []aiStreamMessage `json:"messages"`
	Temperature float64           `json:"temperature"`
	Stream      bool              `json:"stream"`
}

// aiStreamChunk SSE 流式响应单帧：data: {"choices":[{"delta":{"content":"xxx"}}]}
type aiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// CallLLMStream 调用豆包 AI（SSE 流式输出），每收到一段内容立即回调 callback(chunk)。
// 通过命名返回值 + defer 与 CallLLM 保持一致的指标上报（Prometheus + Redis dashboard 计数）。
// ctx 由调用方（handler）传入，客户端断开时自动取消上游 HTTP 请求。
func CallLLMStream(ctx context.Context, prompt string, callback func(string) error) (err error) {
	// 业务指标：AI 调用次数 / 失败次数（与 CallLLM 一致）
	defer func() {
		metrics.RecordAICall(err == nil)
		if err == nil {
			IncrDashboardCounter(DashboardMetricAICalls)
		} else {
			IncrDashboardCounter(DashboardMetricAIErrors)
		}
	}()

	// 从 config 层获取 AI 配置（环境变量读取，复用 CallLLM 同一套配置）
	apiKey := config.AIAPIKey()
	model := config.AIModel()
	apiURL := config.AIURL()
	timeout := config.AITimeout()

	// 1. 构建流式请求体（system 提示词 + 用户 prompt + stream: true）
	payload := aiStreamRequest{
		Model: model,
		Messages: []aiStreamMessage{
			{Role: "system", Content: aiSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: aiTemperature,
		Stream:      true,
	}

	// 2. JSON 序列化
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("AI 流式请求序列化失败: %w", err)
	}

	// 3. 继承调用方 context（客户端断开即取消），并叠加超时
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 4. 创建带 context 的 HTTP Request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建 AI 流式请求失败: %w", err)
	}

	// 5. 设置 Header
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", aiUserAgent)

	// 5.1 网络分层诊断日志：请求发送前输出（与 CallLLM 对齐；DNS / TCP / TLS 在拨号完成后回填）
	aiLogf(`
==== AI NETWORK DEBUG START (STREAM) ====
DNS: (测量中...)
TCP CONNECT: (测量中...)
TLS HANDSHAKE: (测量中...)
HTTP VERSION: HTTP/1.1 (ForceAttemptHTTP2=false)
PROXY: %s
TIME: %s
==== AI NETWORK DEBUG END ====
`,
		aiProxyDesc(),
		time.Now().Format(time.RFC3339),
	)

	// 6. HTTP Client（与 CallLLM 复用同一套诊断 Transport）：
	//    未配置 AI_PROXY 时 Proxy=nil 屏蔽系统环境代理；ForceAttemptHTTP2=false + ALPN http/1.1
	//    明确走 HTTP/1.1，规避部分网络环境（防火墙 / VPN）对 HTTP/2 帧的干扰导致请求挂起。
	httpStart := time.Now()
	client := &http.Client{
		Timeout:   timeout,
		Transport: newAIDebugTransport(),
	}

	resp, err := client.Do(req)
	httpCost := time.Since(httpStart)
	if err != nil {
		dnsCost, tcpCost, tlsCost := readNetTiming()
		aiLogf("AI NETWORK LAYERS (FAIL): DNS=%v TCP=%v TLS=%v\n", dnsCost, tcpCost, tlsCost)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return errors.New("AI 流式服务响应超时，请稍后重试")
		}
		return fmt.Errorf("AI 流式服务请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 6.1 响应头到达后立即打印（SSE 请求核心诊断：HTTP STATUS / HTTP PROTO / SERVER HEADER）
	dnsCost, tcpCost, tlsCost := readNetTiming()
	aiLogf(`==== AI NETWORK DEBUG RESULT (STREAM) ====
DNS: %v
TCP CONNECT: %v
TLS HANDSHAKE: %v
HTTP STATUS: %s
HTTP PROTO: %s
SERVER HEADER: %s
HEADER COST: %v
============================
`, dnsCost, tcpCost, tlsCost, resp.Status, resp.Proto, resp.Header.Get("Server"), httpCost)

	// 7. HTTP 状态码检查（非 2xx 视为 API 错误）
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AI 流式服务返回异常状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 8. SSE 逐行解析：data: {"choices":[{"delta":{"content":"xxx"}}]}
	scanner := bufio.NewScanner(resp.Body)
	// 加大 Scanner 缓冲区：初始 64KB、单行最大 1MB，避免超长帧被截断
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// 只处理 data: 开头的行
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		// 结束标记：data: [DONE]
		if data == "[DONE]" {
			return nil
		}

		// 解析单帧，提取 choices[0].delta.content
		var chunk aiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// 单帧解析失败不影响整体流，跳过继续读取
			continue
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		// 收到内容立即回调
		if cbErr := callback(chunk.Choices[0].Delta.Content); cbErr != nil {
			return fmt.Errorf("AI 流式输出回调失败: %w", cbErr)
		}
	}

	// 9. 读取结束检查
	if err := scanner.Err(); err != nil {
		// 客户端断开导致上游请求被取消，原样返回 context 错误
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("读取 AI 流式响应失败: %w", err)
	}
	return nil
}
