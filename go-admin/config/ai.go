package config

import (
	"os"
	"sync"
	"time"
)

// AI 配置默认值（非敏感项；敏感项必须通过环境变量注入）
const (
	// DefaultAIURL 豆包 Chat Completions API 地址
	DefaultAIURL = "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
	// DefaultAITimeout AI 请求默认超时（秒）
	DefaultAITimeout = 120
	// DefaultAIProxy 自定义代理默认值（空 = 未配置，屏蔽系统环境代理直连；生产需走代理时通过 AI_PROXY 指定）
	DefaultAIProxy = ""
	// DefaultAINetDebugLog AI 网络分层诊断日志默认开启（诊断阶段保持输出；生产环境建议 AI_NETWORK_DEBUG_LOG=false 关闭）
	DefaultAINetDebugLog = true
	// DefaultEnableAINetDebug AI 网络诊断路由默认开启（公网生产环境可设 ENABLE_AI_NET_DEBUG=false 关闭）
	DefaultEnableAINetDebug = true
)

var (
	aiInitOnce       sync.Once
	aiAPIKey         string
	aiEndpoint       string
	aiModel          string
	aiURL            string
	aiProxy          string
	aiTimeout        time.Duration
	aiNetDebugLog    bool
	aiEnableNetDebug bool
)

// InitAI 从环境变量读取 AI 配置（懒加载 + 幂等，可并发安全调用）。
// 不在包 init() 中执行，确保 main() 的 godotenv.Load() 之后仍能正确读取 .env。
func InitAI() {
	aiInitOnce.Do(func() {
		aiAPIKey = os.Getenv("AI_API_KEY")
		aiEndpoint = os.Getenv("AI_ENDPOINT")
		aiModel = getEnv("AI_MODEL", "")
		aiURL = getEnv("AI_URL", DefaultAIURL)
		aiProxy = os.Getenv("AI_PROXY")
		aiTimeout = time.Duration(getEnvInt("AI_TIMEOUT", DefaultAITimeout)) * time.Second
		aiNetDebugLog = getEnvBool("AI_NETWORK_DEBUG_LOG", DefaultAINetDebugLog)
		aiEnableNetDebug = getEnvBool("ENABLE_AI_NET_DEBUG", DefaultEnableAINetDebug)
	})
}

// AIAPIKey 返回豆包 API Key（环境变量 AI_API_KEY；生产必须注入，严禁硬编码）
func AIAPIKey() string { InitAI(); return aiAPIKey }

// AIEndpoint 返回豆包接入点 ID（环境变量 AI_ENDPOINT）
func AIEndpoint() string { InitAI(); return aiEndpoint }

// AIModel 返回请求体 model 字段（环境变量 AI_MODEL；未配置时回退到 AI_ENDPOINT，保持向后兼容）
func AIModel() string {
	InitAI()
	if aiModel != "" {
		return aiModel
	}
	return aiEndpoint
}

// AIURL 返回豆包 API 地址（环境变量 AI_URL，默认火山方舟 Chat Completions）
func AIURL() string { InitAI(); return aiURL }

// AITimeout 返回 AI 请求超时（环境变量 AI_TIMEOUT，单位秒，默认 120）
func AITimeout() time.Duration { InitAI(); return aiTimeout }

// AIProxy 返回自定义代理地址（环境变量 AI_PROXY；空 = 未配置，Transport 将屏蔽系统环境代理直连）
func AIProxy() string { InitAI(); return aiProxy }

// AINetworkDebugLog 是否输出 AI 网络分层诊断日志（环境变量 AI_NETWORK_DEBUG_LOG，默认 true；生产建议 false）
func AINetworkDebugLog() bool { InitAI(); return aiNetDebugLog }

// EnableAINetDebug 是否注册 AI 网络诊断路由（环境变量 ENABLE_AI_NET_DEBUG，默认 true；公网生产建议 false）
func EnableAINetDebug() bool { InitAI(); return aiEnableNetDebug }
