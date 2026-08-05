/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 应用标题 */
  readonly VITE_APP_TITLE?: string
  /** API 基础路径（默认 /api，开发环境经 Vite 代理转发到 Go 后端） */
  readonly VITE_API_BASE_URL?: string
  /** 开发环境代理目标（默认 http://localhost:8080，见 vite.config.ts） */
  readonly VITE_PROXY_TARGET?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
