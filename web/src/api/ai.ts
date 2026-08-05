/**
 * AI 能力相关 API（见 go-admin/docs/API.md 第 3.8~3.11 节，均需 JWT）
 *
 *   - POST /ai/generate          代码生成（请求体：{ prompt }）→ 返回 Markdown 文本
 *   - POST /ai/generate/stream   代码生成 · SSE 流式输出（请求体：{ prompt }）
 *   - POST /ai/explain           代码解释（请求体：{ code }）→ 返回 Markdown 文本
 *   - POST /ai/fix               代码修复（请求体：{ code }）→ 返回 Markdown 文本
 *   - POST /ai/optimize          代码优化（请求体：{ code }）→ 返回 Markdown 文本
 *
 * 约定：
 *   - 普通接口统一走 src/utils/request.ts（axios：自动携带 Authorization: Bearer
 *     <accessToken>，并解包 { code, msg, data } 响应信封）
 *   - SSE 流式接口不使用 axios（axios 对 SSE 支持不好），改用原生 fetch + ReadableStream：
 *       · 手动携带 Authorization: Bearer <accessToken>
 *       · 逐帧解析 text/event-stream，帧格式：
 *           data: {"content":"<chunk>"}   内容帧 → 回调 onChunk
 *           data: {"error":"<msg>"}       错误帧 → 抛出 Error
 *           data: [DONE]                  正常结束标记
 *           : ping                        心跳注释帧 → 忽略（":" 开头为 SSE 注释，不解析为 JSON）
 *       · 心跳帧（": ping"）仅用于防止长空闲断流，不影响任何业务数据
 */
import { request } from '@/utils/request'
import { clearTokens, getAccessToken } from '@/utils/auth'

import type { AIResultData, CodeParams, GenerateCodeParams } from '@/types/ai'

/** 代码生成：POST /api/ai/generate（请求体：{ prompt }） */
export function generateCode(params: GenerateCodeParams): Promise<AIResultData> {
  return request<AIResultData>({ url: '/ai/generate', method: 'post', data: params })
}

/** 代码解释：POST /api/ai/explain（请求体：{ code }） */
export function explainCode(params: CodeParams): Promise<AIResultData> {
  return request<AIResultData>({ url: '/ai/explain', method: 'post', data: params })
}

/** 代码修复：POST /api/ai/fix（请求体：{ code }） */
export function fixCode(params: CodeParams): Promise<AIResultData> {
  return request<AIResultData>({ url: '/ai/fix', method: 'post', data: params })
}

/** 代码优化：POST /api/ai/optimize（请求体：{ code }） */
export function optimizeCode(params: CodeParams): Promise<AIResultData> {
  return request<AIResultData>({ url: '/ai/optimize', method: 'post', data: params })
}

// ==================== SSE 流式生成（fetch + ReadableStream） ====================

export interface GenerateStreamOptions {
  /** 每收到一段内容立即回调（用于实时追加显示） */
  onChunk?: (chunk: string) => void
  /** 中断信号（AbortController.signal），用于取消流式请求 */
  signal?: AbortSignal
}

/** API 基础路径（与 src/utils/request.ts 保持一致，开发环境经 Vite 代理转发到 Go 后端） */
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'

/**
 * 代码生成（SSE 流式）：POST /api/ai/generate/stream（请求体：{ prompt }）
 *
 * 使用 fetch + ReadableStream：
 *   1. 携带 Authorization: Bearer <accessToken>
 *   2. 读取 response.body.getReader()，按空行切分 SSE 事件帧
 *   3. 每收到一个内容帧立即回调 onChunk(chunk)，实现实时显示
 * 失败（HTTP 非 2xx / SSE error 帧 / 网络异常）时抛出 Error。
 */
export async function generateStream(
  params: GenerateCodeParams,
  options: GenerateStreamOptions = {},
): Promise<void> {
  const { onChunk, signal } = options

  const token = getAccessToken()
  const response = await fetch(`${API_BASE_URL}/ai/generate/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(params),
    signal,
  })

  // 非 2xx：提取后端错误提示；401 同步清空登录态并回登录页
  if (!response.ok) {
    const message = await extractErrorMessage(response)
    if (response.status === 401) {
      clearTokens()
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    throw new Error(message)
  }

  if (!response.body) {
    throw new Error('当前浏览器不支持流式响应')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    // 按空行切分 SSE 事件帧；末段可能是跨 chunk 的不完整帧，留到下一轮
    const frames = buffer.split(/\r?\n\r?\n/)
    buffer = frames.pop() ?? ''
    for (const frame of frames) {
      if (handleSSEFrame(frame, onChunk)) return
    }
  }

  // 处理流结束后可能残留的最后一段（无空行结尾的帧）
  buffer += decoder.decode()
  if (buffer.trim() && handleSSEFrame(buffer, onChunk)) return
}

/**
 * 解析单个 SSE 事件帧：提取 data: 行并处理。
 * 心跳注释帧（仅含 ":" 开头的行，如 ": ping"）提取后 data 为空，直接忽略，不进入 JSON 解析。
 * 返回 true 表示收到结束标记 [DONE]；遇错误帧直接抛出 Error。
 */
function handleSSEFrame(frame: string, onChunk?: (chunk: string) => void): boolean {
  const data = extractSSEData(frame).trim()
  // 空 data：心跳注释帧 / 纯空帧，忽略（不解析为 JSON）
  if (!data) return false

  if (data === '[DONE]') return true

  let payload: { content?: string; error?: string }
  try {
    payload = JSON.parse(data) as { content?: string; error?: string }
  } catch {
    // 非 JSON 的异常帧：忽略，保证流不中断
    return false
  }

  if (payload.error) {
    throw new Error(payload.error)
  }
  if (typeof payload.content === 'string' && payload.content) {
    onChunk?.(payload.content)
  }
  return false
}

/** 提取 SSE 帧中的 data: 行内容（支持多行 data: 拼接；":" 开头的注释/心跳行忽略） */
function extractSSEData(frame: string): string {
  const dataLines: string[] = []
  for (const line of frame.split(/\r?\n/)) {
    // 心跳注释行（":" 开头，如 ": ping"）：按 SSE 规范忽略，不参与 data 拼接 / JSON 解析
    if (line.startsWith(':')) continue
    if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart())
    }
  }
  return dataLines.join('\n')
}

/** 从非 2xx 响应中提取错误提示：JSON 信封优先，其次 SSE error 帧，最后兜底状态文本 */
async function extractErrorMessage(response: Response): Promise<string> {
  const fallback = `请求失败（HTTP ${response.status}）`

  let text = ''
  try {
    text = await response.text()
  } catch {
    return fallback
  }
  if (!text) return fallback

  // 1) 常规 JSON 信封：{ "msg": "..." }
  try {
    const json = JSON.parse(text) as { msg?: string; error?: string }
    if (json.msg) return json.msg
    if (json.error) return json.error
  } catch {
    // 不是 JSON，继续尝试 SSE error 帧
  }

  // 2) SSE error 帧：data: {"error":"..."}
  try {
    const data = extractSSEData(text).trim()
    if (data) {
      const payload = JSON.parse(data) as { error?: string }
      if (payload.error) return payload.error
    }
  } catch {
    // 忽略
  }

  return fallback
}
