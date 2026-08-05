package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"go-admin/internal/dto"
	"go-admin/internal/repository"
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

// ==================== 生成代码（SSE 流式输出） ====================

// aiSSEHeartbeatInterval SSE 心跳间隔：上游长时间不返回 token（思考中 / 冷启动）时，
// 每 15 秒发送一次注释帧 ": ping"，防止客户端或中间代理（Nginx / 负载均衡）因长时间
// 无数据而判定连接超时断流。注释帧不改变现有 SSE 协议（data: {"content":...} / data: [DONE]）。
const aiSSEHeartbeatInterval = 15 * time.Second

// GenerateStream AI 生成代码（SSE 流式输出）
// @Summary AI 生成代码（SSE 流式输出）
// @Description AI SSE Streaming API：根据需求描述流式生成代码；SSE 事件格式为 data: {"content":"<chunk>"}，每次输出后立即 Flush，流结束发送 data: [DONE]
// @Tags AI
// @Accept json
// @Produce text/event-stream
// @Security BearerAuth
// @Param request body dto.GenerateCodeReq true "生成代码请求"
// @Success 200 {string} string "SSE 流（text/event-stream）"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "AI 服务异常"
// @Router /ai/generate/stream [post]
func GenerateStream(c *gin.Context) {
	var req dto.GenerateCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeSSEHeaders(c)
		c.Status(http.StatusBadRequest)
		_ = writeSSEEvent(c, map[string]string{"error": "参数错误"})
		return
	}

	// 设置 SSE 响应头（必须在写入任何内容之前）
	writeSSEHeaders(c)

	ctx := c.Request.Context()

	// ===== SSE 心跳机制 =====
	// 上游大模型长时间不返回 token 时，若连接长时间无任何字节，客户端 / 中间代理可能
	// 判定连接超时而断流。方案：time.Ticker 每 15 秒发送一次 SSE 注释帧 ": ping"
	// （SSE 规范：以 ":" 开头的行是注释，客户端必须忽略），并立即 Flush。
	// 心跳帧与 content 帧通过互斥锁串行写入，保证帧边界完整、不影响正常 chunk 输出。
	var mu sync.Mutex // 保护 c.Writer 并发写入（chunk 与 ping 互斥）
	ticker := time.NewTicker(aiSSEHeartbeatInterval)
	defer ticker.Stop()

	stopHeartbeat := make(chan struct{})    // 通知心跳 goroutine 退出（流结束）
	heartbeatStopped := make(chan struct{}) // 心跳 goroutine 已退出
	go func() {
		defer close(heartbeatStopped)
		for {
			select {
			case <-ctx.Done():
				// 客户端已断开：立即退出，不再发送任何内容
				return
			case <-stopHeartbeat:
				return
			case <-ticker.C:
				mu.Lock()
				if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
					// 写失败（通常客户端已断开）：退出心跳循环
					mu.Unlock()
					return
				}
				c.Writer.Flush()
				mu.Unlock()
			}
		}
	}()

	streamErr := service.GenerateStream(ctx, req.Prompt, func(chunk string) error {
		// 客户端已断开（ctx 取消）：立即停止后续输出
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		mu.Lock()
		defer mu.Unlock()
		return writeSSEEvent(c, map[string]string{"content": chunk})
	})

	// 停止心跳 goroutine 并等待其退出，避免后续 [DONE] / error 帧与 ping 并发写入
	close(stopHeartbeat)
	<-heartbeatStopped

	if streamErr != nil {
		// 流式输出中途失败：发送错误事件帧（与 content 帧格式对称）
		_ = writeSSEEvent(c, map[string]string{"error": streamErr.Error()})
		return
	}

	// 正常结束标记
	_, _ = c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

// writeSSEHeaders 设置 SSE 响应头（必须在写入任何内容之前调用）
func writeSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
}

// writeSSEEvent 写入一个 SSE 事件帧：data: <json>\n\n，并立即 Flush。
// 返回错误用于中断流式循环（写失败通常意味着客户端已断开）。
func writeSSEEvent(c *gin.Context, obj map[string]string) error {
	payload, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	if _, err := c.Writer.WriteString("data: " + string(payload) + "\n\n"); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

// ==================== AI 网络诊断 ====================

// DebugNetwork AI 网络诊断测试接口
// @Summary AI 网络诊断
// @Description 测试到火山方舟的网络连通性：DNS / TCP / TLS / HTTP GET，耗时单位为毫秒数值（dns_ms / tcp_ms / tls_ms / total_ms），便于监控阈值告警
// @Tags AI
// @Produce json
// @Success 200 {object} response.Result "诊断结果"
// @Router /ai/debug/network [get]
func DebugNetwork(c *gin.Context) {
	result := repository.NetworkDebug()
	response.Success200(c, result)
}
