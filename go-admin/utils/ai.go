package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// ============ 请把下面两行替换成你自己的 Key 和接入点 ID ============
const (
	API_KEY  = "ark-9919935b-8e37-437c-8e66-b98e6f3ae402-24210" // 替换为 ek- 开头的字符串
	ENDPOINT = "ep-20260422210455-x24rs"                        // 替换为 ep- 开头的字符串
	API_URL  = "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type AIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// 调用豆包AI（支持联网插件，适配代码学习场景）
func CallAI(prompt string) (string, error) {
	req := AIRequest{
		Model: ENDPOINT,
		Messages: []Message{
			{
				Role:    "system",
				Content: "你是专业代码学习助手，擅长代码生成、解释、修复与优化，输出简洁、可直接运行的内容，必要时可使用联网插件获取最新信息。",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.3, // 调低温度，让代码输出更稳定
	}

	jsonData, _ := json.Marshal(req)
	request, _ := http.NewRequest("POST", API_URL, bytes.NewBuffer(jsonData))
	request.Header.Set("Authorization", "Bearer "+API_KEY)
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result AIResponse
	json.Unmarshal(body, &result)

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "AI 服务繁忙，请稍后再试", nil
}
