# go-admin Web 前端

面向 `go-admin`（Go + Gin）后端的企业后台管理系统前端，独立运行，不依赖后端代码。

## 技术栈

- Vue 3（`<script setup>` + TypeScript）
- Vite 6
- Element Plus（中文语言包）
- Pinia（状态管理）
- Vue Router 4
- Axios（统一请求封装）
- Sass（样式）

## 环境要求

- Node.js >= 18
- Go 后端运行在 `http://localhost:8080`

## 快速开始

```bash
cd web
npm install
npm run dev        # http://localhost:5173
```

| 命令 | 说明 |
|------|------|
| `npm run dev`        | 启动开发服务器（热更新） |
| `npm run build`      | 类型检查 + 生产构建 |
| `npm run preview`    | 预览生产构建产物 |
| `npm run type-check` | 仅做 TS / Vue 类型检查 |

## 开发环境代理

`vite.config.ts` 已将以下前缀代理到 Go 后端，前端无需处理跨域：

| 前缀 | 目标 |
|------|------|
| `/api`    | `http://localhost:8080` |
| `/swagger` | `http://localhost:8080` |

如需覆盖（例如后端不在本机），在 `web/.env.local` 中配置（该文件已被 gitignore）：

```
VITE_PROXY_TARGET=http://localhost:8080
VITE_API_BASE_URL=/api
```

> 安全约定：与 go-admin 保持一致，`.env*` 一律禁止入库（仓库 CI 安全门禁会拦截）。

## 目录结构

```
web/
├── public/                  # 静态资源（favicon 等）
├── src/
│   ├── api/                 # 接口请求层（auth / user / ai）
│   ├── assets/styles/       # 全局样式（变量 + 基础样式）
│   ├── components/          # 通用业务组件（预留）
│   ├── composables/         # 组合式函数（预留）
│   ├── directives/          # 自定义指令，如 v-permission（预留）
│   ├── layout/              # 主布局（侧边栏 / 顶栏 / 内容区）
│   ├── router/              # 路由
│   ├── stores/              # Pinia（user / app）
│   ├── types/               # TS 类型（对接后端响应信封 / DTO）
│   ├── utils/               # 工具（axios 封装 / token 存取）
│   └── views/               # 页面（login / dashboard / user / ai / error）
├── index.html
├── package.json
├── tsconfig.json / tsconfig.node.json
└── vite.config.ts
```

## 与后端对接约定（详见 go-admin/docs/API.md）

- 统一响应信封：`{ "code": number, "msg": string, "data": any }`
  - 成功：`code = 0`（AI 模块为 `code = 200`）
  - 失败：`code` 与 HTTP 状态码一致（400 / 401 / 404 / 429 / 500）
- 鉴权方式：`Authorization: Bearer <accessToken>`
- 登录返回 JWT 双 Token：`{ accessToken, refreshToken, accessJTI, refreshJTI }`
  - access_token 有效期 15 分钟；refresh_token 有效期 7 天（刷新时旋转作废旧 token）
- 接口调试：`http://localhost:8080/swagger/index.html`

## 当前进度

- [x] 项目初始化（Vite + TS + Element Plus + Pinia + Router + Axios + Sass）
- [x] 目录设计与可运行骨架（主布局 / 路由 / 占位页面）
- [x] 开发代理到 Go 后端
- [ ] 登录页面 + JWT Token 存取（`utils/auth.ts` / `stores/modules/user.ts` / `views/login`）
- [ ] Axios 拦截器（自动携带 Bearer Token、统一错误处理、401 刷新）（`utils/request.ts`）
- [ ] 用户管理页面（列表 / 编辑 / 删除 / 状态切换）（`views/user`）
- [ ] AI 助手页面（生成 / 解释 / 修复 / 优化）（`views/ai`）

> 旧版单文件原型保留在 `web/prototype/index.html`，可作下一阶段业务实现参考（注意其接口路径与当前后端 REST 契约略有差异）。
