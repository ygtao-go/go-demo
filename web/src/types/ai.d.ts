/**
 * AI 模块类型定义
 *
 * 与后端完全对齐（见 go-admin/docs/API.md 第 3.8~3.11 节、go-admin/internal/dto/ai.go）：
 *   - POST /api/ai/generate  请求体：{ prompt }（dto.GenerateCodeReq，prompt 必填）
 *   - POST /api/ai/explain   请求体：{ code }   （dto.CodeReq，code 必填）
 *   - POST /api/ai/fix       请求体：{ code }   （dto.CodeReq，code 必填）
 *   - POST /api/ai/optimize  请求体：{ code }   （dto.CodeReq，code 必填）
 *
 * 成功响应（业务码固定 200，兼容 web 前端 data.code === 200 判断，见 pkg/response/Result.go）：
 *   { "code": 200, "msg": "success", "data": "<Markdown 文本>" }
 * 即：AI 模块 data 为纯字符串（Markdown 格式），由前端渲染为代码块 / 标题 / 列表。
 */

/** AI 代码生成请求体（dto.GenerateCodeReq，prompt 必填） */
export interface GenerateCodeParams {
  prompt: string
}

/** AI 代码解释 / 修复 / 优化请求体（dto.CodeReq，code 必填） */
export interface CodeParams {
  code: string
}

/**
 * AI 模式（与后端 4 个 AI 接口一一对应）：
 *   - generate  代码生成 → POST /api/ai/generate
 *   - explain   代码解释 → POST /api/ai/explain
 *   - fix       代码修复 → POST /api/ai/fix
 *   - optimize  代码优化 → POST /api/ai/optimize
 */
export type AIMode = 'generate' | 'explain' | 'fix' | 'optimize'

/**
 * AI 接口成功响应数据：后端返回 Markdown 格式文本字符串
 * （response.Success200 → data 字段，类型为 string）。
 */
export type AIResultData = string
