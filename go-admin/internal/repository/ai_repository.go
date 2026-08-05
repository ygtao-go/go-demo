package repository

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

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

// ==================== AI 网络诊断 Transport（CallLLM / CallLLMStream / NetworkDebug 共用） ====================
// 诊断目标：让 Go HTTP Client 行为与 PowerShell（HTTP/1.1 + 直连，5s 返回）对齐。
// 历史现象：Go 默认 http.DefaultTransport 在部分网络环境下 90s→429 / 120s→超时。
// 修复要点：
//
//	① Proxy 分层策略 —— 配置了 AI_PROXY 走自定义代理；未配置则 Proxy=nil 屏蔽系统环境代理
//	  （HTTP_PROXY / HTTPS_PROXY / ALL_PROXY），避免 Go 隐式走代理隧道导致请求挂起；
//	② ForceAttemptHTTP2=false + ALPN 仅通告 http/1.1 —— 强制 HTTP/1.1，
//	  规避中间设备（防火墙 / VPN）干扰 HTTP/2 帧导致请求挂起（典型症状：Go 超时、PowerShell 正常）；
//	③ DialContext / DialTLSContext 手动拆分 DNS → TCP → TLS 三层耗时并封装分层错误，
//	  精准定位卡在哪个网络层；
//	④ TLSHandshakeTimeout 30s（握手不可能超过 30s，快速失败）；
//	⑤ ResponseHeaderTimeout 对齐总超时（120s）—— 长文本大模型冷启动时服务端可能 >30s 才返回响应头，
//	  与 120s 总超时目标不冲突；
//	⑥ ExpectContinueTimeout 1s —— 保留，用于前置校验优化；
//	⑦ 连接复用控制 MaxIdleConns=20 / IdleConnTimeout=30s —— 避免长连接闲置卡死。
const (
	// aiNetworkTimeout 网络握手类操作单层最长等待（DNS / TCP / TLS）
	aiNetworkTimeout = 30 * time.Second
	// aiUserAgent 统一 User-Agent，便于火山方舟服务端识别调用方
	aiUserAgent = "go-admin-ai-client/1.0"
)

// aiNetTiming 记录最近一次 AI 请求拨号的分层耗时（DNS / TCP / TLS），供请求结束日志输出。
// 并发调用（CallLLM 与 CallLLMStream 同时进行）时仅用于诊断展示，允许轻微串扰。
var aiNetTiming struct {
	sync.Mutex
	dnsCost time.Duration
	tcpCost time.Duration
	tlsCost time.Duration
}

func recordNetTiming(dnsCost, tcpCost, tlsCost time.Duration) {
	aiNetTiming.Lock()
	aiNetTiming.dnsCost = dnsCost
	aiNetTiming.tcpCost = tcpCost
	aiNetTiming.tlsCost = tlsCost
	aiNetTiming.Unlock()
}

func readNetTiming() (dnsCost, tcpCost, tlsCost time.Duration) {
	aiNetTiming.Lock()
	defer aiNetTiming.Unlock()
	return aiNetTiming.dnsCost, aiNetTiming.tcpCost, aiNetTiming.tlsCost
}

// aiLogf 统一网络诊断日志开关：AI_NETWORK_DEBUG_LOG=true 时输出详细分层耗时日志（生产可关闭）。
func aiLogf(format string, args ...interface{}) {
	if config.AINetworkDebugLog() {
		fmt.Printf(format, args...)
	}
}

// aiProxyDesc 返回当前代理策略描述（供日志输出）。
func aiProxyDesc() string {
	if p := config.AIProxy(); p != "" {
		return p
	}
	return "disabled (无自定义代理, 系统环境代理已屏蔽)"
}

// newAIDebugTransport 构建带网络诊断的自定义 Transport（CallLLM / CallLLMStream / NetworkDebug 共用）。
func newAIDebugTransport() *http.Transport {
	// 代理分层：配置 AI_PROXY 走自定义代理；未配置保持 nil（屏蔽系统环境代理，直连）。
	var proxyFunc func(*http.Request) (*url.URL, error)
	if p := config.AIProxy(); p != "" {
		proxyURL, err := url.Parse(p)
		if err != nil {
			fmt.Printf("AI PROXY CONFIG INVALID: %q, 回退为禁用代理直连\n", p)
		} else {
			proxyFunc = http.ProxyURL(proxyURL)
		}
	}

	return &http.Transport{
		Proxy:                 proxyFunc,
		ForceAttemptHTTP2:     false,
		TLSHandshakeTimeout:   aiNetworkTimeout,
		ResponseHeaderTimeout: config.AITimeout(),
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           aiMeasuredDialContext,
		DialTLSContext:        aiMeasuredDialTLSContext,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
	}
}

// aiMeasuredDialContext 手动拆分 DNS 与 TCP 建立时间：
//  1. net.DefaultResolver.LookupIPAddr 解析域名并记录 DNS 耗时；
//  2. 遍历解析出的 IP 逐个建立 TCP 连接并记录 TCP 耗时。
//
// 任一层失败均返回带层级标记的错误（AI DNS ERROR / AI TCP CONNECT ERROR），便于日志区分网络故障层级。
func aiMeasuredDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// 非 host:port 形式：退化到标准拨号
		return (&net.Dialer{Timeout: aiNetworkTimeout, KeepAlive: 30 * time.Second}).DialContext(ctx, network, addr)
	}

	// ---- DNS ----
	dnsStart := time.Now()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	dnsCost := time.Since(dnsStart)
	if err != nil {
		recordNetTiming(dnsCost, 0, 0)
		return nil, fmt.Errorf("AI DNS ERROR: 解析 %s 失败: %w", host, err)
	}
	if len(ips) == 0 {
		recordNetTiming(dnsCost, 0, 0)
		return nil, fmt.Errorf("AI DNS ERROR: 解析 %s 无结果", host)
	}

	// ---- TCP ----
	dialer := &net.Dialer{Timeout: aiNetworkTimeout, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		target := net.JoinHostPort(ip.IP.String(), port)
		tcpStart := time.Now()
		conn, dialErr := dialer.DialContext(ctx, network, target)
		tcpCost := time.Since(tcpStart)
		recordNetTiming(dnsCost, tcpCost, 0)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, fmt.Errorf("AI TCP CONNECT ERROR: %s: %w", addr, lastErr)
}

// aiMeasuredDialTLSContext 在已建立的 TCP 连接之上手动执行 TLS 握手并记录耗时：
// ALPN 只通告 http/1.1，进一步杜绝协商出 HTTP/2（与 ForceAttemptHTTP2=false 双保险）。
func aiMeasuredDialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	rawConn, err := aiMeasuredDialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"}, // 只协商 HTTP/1.1，杜绝 h2
	})

	// TLS 握手（30s 超时，与 TLSHandshakeTimeout 一致）
	handshakeCtx, cancel := context.WithTimeout(ctx, aiNetworkTimeout)
	defer cancel()
	tlsStart := time.Now()
	if err := tlsConn.HandshakeContext(handshakeCtx); err != nil {
		_ = rawConn.Close()
		dnsCost, tcpCost, _ := readNetTiming()
		recordNetTiming(dnsCost, tcpCost, time.Since(tlsStart))
		return nil, fmt.Errorf("AI TLS HANDSHAKE FAILED: %s: %w", addr, err)
	}
	dnsCost, tcpCost, _ := readNetTiming()
	recordNetTiming(dnsCost, tcpCost, time.Since(tlsStart))
	return tlsConn, nil
}

// init 启动时打印代理相关环境变量，用于确认是否存在系统环境代理。
// 注意：此处先于 main() 的 godotenv.Load() 执行（包 init 早于 main 包 init），
// 因此只能读取原始 OS 环境变量，不能触发 config.InitAI()（否则 .env 尚未加载会读到空值）。
// 自定义 AI_PROXY（来自 .env）的实际生效值由请求时的 aiProxyDesc() / newAIDebugTransport() 输出。
func init() {
	fmt.Printf("AI PROXY CHECK: HTTP_PROXY=%q HTTPS_PROXY=%q ALL_PROXY=%q NO_PROXY=%q\n",
		os.Getenv("HTTP_PROXY"),
		os.Getenv("HTTPS_PROXY"),
		os.Getenv("ALL_PROXY"),
		os.Getenv("NO_PROXY"),
	)
}

// CallLLM 调用豆包 AI（支持联网插件，适配代码学习场景），返回生成的文本。
// 通过命名返回值 + defer 统一上报 AI 调用次数/失败次数业务指标，不改动任何返回语义。
func CallLLM(prompt string) (result string, err error) {
	// 业务指标：AI 调用次数 / 失败次数
	// 保留 Prometheus 指标（metrics.RecordAICall），额外同步 Redis dashboard 业务计数：
	//   成功 → dashboard:ai_calls +1；失败 → dashboard:ai_errors +1
	defer func() {
		metrics.RecordAICall(err == nil)
		if err == nil {
			IncrDashboardCounter(DashboardMetricAICalls)
		} else {
			IncrDashboardCounter(DashboardMetricAIErrors)
		}
	}()

	// 从 config 层获取 AI 配置（环境变量读取）
	apiKey := config.AIAPIKey()
	model := config.AIModel()
	apiURL := config.AIURL()
	timeout := config.AITimeout()

	fmt.Printf(`
========== AI DEBUG ==========
URL: %s
MODEL: %s
KEY LENGTH: %d
TIMEOUT: %v
==============================
`,
		apiURL,
		model,
		len(apiKey),
		timeout,
	)

	// 1. 构建请求体（system 提示词 + 用户 prompt）
	payload := aiRequest{
		Model: model,
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
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", aiUserAgent)

	// 5.1 诊断日志：请求发送前输出（URL / MODEL / BODY）
	fmt.Printf("AI REQUEST START\nURL: %s\nMODEL: %s\nBODY: %s\n", apiURL, model, string(jsonData))

	// 5.2 网络分层诊断日志：请求发送前输出（DNS / TCP / TLS 在拨号完成后回填）
	aiLogf(`
==== AI NETWORK DEBUG START ====
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

	// 6. HTTP Client（显式 Transport + 双保险超时：context.WithTimeout + client.Timeout）
	//
	// 修复说明：
	//   原实现 http.Client{Timeout: timeout} 依赖 http.DefaultTransport，存在两个隐患：
	//     ① 默认协商 HTTP/2（ALPN h2）。部分网络环境（防火墙 / VPN / 运营商中间设备）会干扰
	//        HTTP/2 帧，导致请求挂起直到超时 —— 典型症状是 Go 超时、而 PowerShell/curl（HTTP/1.1）正常。
	//     ② 代理行为隐式继承环境变量（HTTP_PROXY / HTTPS_PROXY / NO_PROXY），
	//        与 PowerShell 使用的 WinINET 系统代理不一致，特定网络下会连错出口导致挂起。
	//   改为显式 newAIDebugTransport()：
	//     ① 未配置 AI_PROXY 时 Proxy=nil —— 屏蔽系统环境代理，与 PowerShell 直连行为一致；
	//     ② 配置 AI_PROXY 时走自定义代理 —— 兼容线上必须通过代理访问方舟的生产环境；
	//     ③ ForceAttemptHTTP2=false + ALPN 仅通告 http/1.1 —— 明确走 HTTP/1.1，规避 h2 帧被干扰的挂起；
	//     ④ Dial 分层测量 DNS / TCP / TLS 并封装分层错误 —— 若仍失败，可快速定位到具体网络层。
	client := &http.Client{
		Timeout:   timeout,
		Transport: newAIDebugTransport(),
	}

	// 请求耗时计时（AI HTTP COST 使用）
	httpStart := time.Now()

	resp, err := client.Do(req)
	httpCost := time.Since(httpStart)
	if err != nil {
		// 诊断日志：请求结束（失败）+ 完整 error
		fmt.Printf("AI HTTP COST: %v\nAI ERROR: %v\n", httpCost, err)
		dnsCost, tcpCost, tlsCost := readNetTiming()
		aiLogf("AI NETWORK LAYERS (FAIL): DNS=%v TCP=%v TLS=%v\n", dnsCost, tcpCost, tlsCost)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "", errors.New("AI 服务响应超时，请稍后重试")
		}
		return "", fmt.Errorf("AI 服务请求失败: %w", err)
	}
	// 诊断日志：请求结束（成功）
	fmt.Printf("AI HTTP COST: %v (HTTP %d)\n", httpCost, resp.StatusCode)
	dnsCost, tcpCost, tlsCost := readNetTiming()
	aiLogf(`==== AI NETWORK DEBUG RESULT ====
DNS: %v
TCP CONNECT: %v
TLS HANDSHAKE: %v
HTTP STATUS: %s
HTTP PROTO: %s
SERVER HEADER: %s
TOTAL COST: %v
============================
`, dnsCost, tcpCost, tlsCost, resp.Status, resp.Proto, resp.Header.Get("Server"), httpCost)
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

// ==================== AI 网络诊断测试接口 ====================

// NetworkDebugResult AI 网络诊断结果（耗时统一为 time.Duration 数值，JSON 输出毫秒级数值，
// 便于自动化监控 / 告警直接做阈值判断；前端展示时自行拼接 "xx ms"）。
type NetworkDebugResult struct {
	DNS     time.Duration `json:"dns_ms"`
	TCP     time.Duration `json:"tcp_ms"`
	TLS     time.Duration `json:"tls_ms"`
	Status  string        `json:"status"`
	Proto   string        `json:"proto"`
	TotalMs time.Duration `json:"total_ms"`
	Server  string        `json:"server_header"`
}

// NetworkDebug 网络诊断：依次测量 DNS → TCP → TLS，并执行一次 GET https://ark.cn-beijing.volces.com。
// 内部使用 30s 超时上下文，防止自检接口自身卡死服务。
// 对应测试接口：GET /api/ai/debug/network
func NetworkDebug() NetworkDebugResult {
	result := NetworkDebugResult{}
	const target = "ark.cn-beijing.volces.com"
	const port = "443"
	const urlStr = "https://" + target

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	totalStart := time.Now()

	// 1. DNS 解析
	dnsStart := time.Now()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, target)
	result.DNS = time.Since(dnsStart)
	if err != nil {
		result.Status = "AI DNS ERROR: " + err.Error()
		result.TotalMs = time.Since(totalStart)
		return result
	}
	if len(ips) == 0 {
		result.Status = "AI DNS ERROR: 无解析结果"
		result.TotalMs = time.Since(totalStart)
		return result
	}

	// 2. TCP 连接（取第一个解析 IP）
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	tcpStart := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ips[0].IP.String(), port))
	result.TCP = time.Since(tcpStart)
	if err != nil {
		result.Status = "AI TCP CONNECT ERROR: " + err.Error()
		result.TotalMs = time.Since(totalStart)
		return result
	}

	// 3. TLS 握手（ALPN 只通告 http/1.1，与 AI 请求行为一致）
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: target,
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"},
	})
	tlsStart := time.Now()
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		result.TLS = time.Since(tlsStart)
		result.Status = "AI TLS HANDSHAKE FAILED: " + err.Error()
		_ = conn.Close()
		result.TotalMs = time.Since(totalStart)
		return result
	}
	result.TLS = time.Since(tlsStart)
	_ = tlsConn.Close() // 释放连接

	// 4. HTTP GET（与 AI 请求相同的 Transport：HTTP/1.1 + 代理策略一致）
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err == nil {
		req.Header.Set("User-Agent", aiUserAgent)
		req.Header.Set("Accept", "text/event-stream")
		resp, doErr := (&http.Client{
			Timeout:   30 * time.Second,
			Transport: newAIDebugTransport(),
		}).Do(req)
		if doErr != nil {
			result.Status = "AI HTTP GET ERROR: " + doErr.Error()
		} else {
			result.Status = resp.Status
			result.Proto = resp.Proto
			result.Server = resp.Header.Get("Server")
			_ = resp.Body.Close()
		}
	} else {
		result.Status = "AI HTTP REQUEST CREATE ERROR: " + err.Error()
	}
	result.TotalMs = time.Since(totalStart)
	return result
}
