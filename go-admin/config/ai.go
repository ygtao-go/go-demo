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
	DefaultAITimeout = 30
)

var (
	aiInitOnce sync.Once
	aiAPIKey   string
	aiEndpoint string
	aiURL      string
	aiTimeout  time.Duration
)

// InitAI 从环境变量读取 AI 配置（懒加载 + 幂等，可并发安全调用）。
// 不在包 init() 中执行，确保 main() 的 godotenv.Load() 之后仍能正确读取 .env。
func InitAI() {
	aiInitOnce.Do(func() {
		aiAPIKey = os.Getenv("AI_API_KEY")
		aiEndpoint = os.Getenv("AI_ENDPOINT")
		aiURL = getEnv("AI_URL", DefaultAIURL)
		aiTimeout = time.Duration(getEnvInt("AI_TIMEOUT", DefaultAITimeout)) * time.Second
	})
}

// AIAPIKey 返回豆包 API Key（环境变量 AI_API_KEY；生产必须注入，严禁硬编码）
func AIAPIKey() string { InitAI(); return aiAPIKey }

// AIEndpoint 返回豆包接入点 ID（环境变量 AI_ENDPOINT）
func AIEndpoint() string { InitAI(); return aiEndpoint }

// AIURL 返回豆包 API 地址（环境变量 AI_URL，默认火山方舟 Chat Completions）
func AIURL() string { InitAI(); return aiURL }

// AITimeout 返回 AI 请求超时（环境变量 AI_TIMEOUT，单位秒，默认 30）
func AITimeout() time.Duration { InitAI(); return aiTimeout }
