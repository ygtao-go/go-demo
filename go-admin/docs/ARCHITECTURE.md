# go-admin 架构说明（ARCHITECTURE）

本文档描述 go-admin 的 **三层架构** 与 **请求流程**。项目已完成从扁平架构到三层架构的迁移，当前所有路由均接入 `internal/` 三层结构，历史扁平 `api/` 包已废弃删除。

## 1. 系统总览

```
┌────────────────────────────────────────────────────────────────────┐
│                          HTTP 客户端                                │
│              （web/index.html / curl / 其他服务）                    │
└──────────────────────────────┬─────────────────────────────────────┘
                               │  :8080
                               ▼
┌────────────────────────────────────────────────────────────────────┐
│                        gin.Engine（cmd/main.go）                    │
│  ── 全局中间件链（注册顺序即执行顺序）──                            │
│  RequestID → Metrics → Logger → RedisLimit → Recovery → Cors       │
│                                                                    │
│  ── 路由（router/router.go）──                                     │
│  GET  /metrics                     （Prometheus，无需 JWT）        │
│  POST /api/user/register|login|refresh （公开组，无需 JWT）        │
│  /api 组 + JWTAuth                  （私有组，需 JWT）             │
│     ├── /user/*        用户模块     └── /ai/*    AI 模块           │
└──────────────────────────────┬─────────────────────────────────────┘
                               ▼
┌────────────────────────────────────────────────────────────────────┐
│  Handler 层（internal/handler）—— HTTP 输入输出，不含业务逻辑       │
│  user.go / ai_handler.go                                           │
└──────────────────────────────┬─────────────────────────────────────┘
                               ▼
┌────────────────────────────────────────────────────────────────────┐
│  Service 层（internal/service）—— 业务规则，不直接访问数据源        │
│  user.go（缓存三防 / 密码校验 / Token 生成与刷新 / 缓存失效）       │
│  ai_service.go（AI 提示词构造）                                     │
└──────────────────────────────┬─────────────────────────────────────┘
                               ▼
┌────────────────────────────────────────────────────────────────────┐
│  Repository 层（internal/repository）—— 数据访问，唯一数据入口      │
│  user.go：MySQL CRUD + Redis 缓存 / JTI 黑名单 / Refresh Token 管理 │
│  ai_repository.go：第三方 AI（火山方舟）HTTP 调用唯一入口            │
└───────┬──────────────────────────────┬──────────────────┬──────────┘
        ▼                              ▼                  ▼
   config.DB（GORM / MySQL）   config.RedisClient     火山方舟 API
        │                          （go-redis）      （Chat Completions）
        └──── 底层依赖均来自 config 层（环境变量初始化，无硬编码）
```

## 2. 三层架构说明

### 2.1 Handler 层（`internal/handler`）

职责：**HTTP 输入输出**。

- 解析请求体/参数为 DTO（`c.ShouldBindJSON` / `c.ShouldBindQuery`），参数错误返回 `400 参数错误`；
- 调用 Service 层方法，透传错误信息；
- 通过 `pkg/response` 输出统一响应信封 `{code, msg, data}`。

文件：

| 文件 | 内容 |
|------|------|
| `internal/handler/user.go` | Register / Login / GetUserInfo / Logout / RefreshToken / UpdatePassword / UserList / EditUser / DeleteUser / SwitchStatus |
| `internal/handler/ai_handler.go` | GenerateCode / ExplainCode / FixCode / OptimizeCode |

### 2.2 Service 层（`internal/service`）

职责：**业务规则**。

- 登录校验（none 缓存短路 → singleflight 加载 DB 密码哈希 → bcrypt 校验）；
- 注册（用户名唯一性 → bcrypt 加密 → 清理残留 none 缓存）；
- 缓存三防编排（穿透 / 击穿 / 雪崩）；
- 双 Token 生成、Refresh Token 旋转与撤销；
- 修改/删除/改状态后的缓存失效；

## 4. 请求流程

### 4.1 登录请求流程（POST /api/user/login）

```
客户端
  │ POST /api/user/login  {username, password}
  ▼
[中间件链] RequestID → Metrics → Logger → RedisLimit → Recovery → Cors
  ▼
handler.Login
  │ 绑定 dto.LoginReq（参数错误 → 400）
  ▼
service.Login(username, password)
  │ ① repository.GetNoneCachedUser —— 命中 none 缓存 → 直接返回“用户不存在”（防穿透）
  │ ② loadUserFromDB（userDBFlight singleflight）
  │       → repository.GetUserByUsername(username)（同一用户名并发仅 1 次 MySQL）
  │       → 存在：CacheUser 回填（TTL 24h ± 1h）
  │       → 不存在：SetNoneCachedUser（TTL 5min）
  │ ③ bcrypt.CompareHashAndPassword —— 校验密码（走 DB 密码哈希，缓存不含密码）
  │ ④ generateTokenResult(userID)
  │       → utils.GenerateAccessToken（15min / tokenType=access / JTI）
  │       → utils.GenerateRefreshToken（7 天 / tokenType=refresh / JTI）
  │       → repository.SaveRefreshJTI —— Redis 同步持久化 refresh JTI（失败则登录失败）
  ▼
response.Success(c, {accessToken, refreshToken, accessJTI, refreshJTI})
```

### 4.2 受保护接口请求流程（如 GET /api/user/info）

```
客户端
  │ GET /api/user/info   Authorization: Bearer <accessToken>
  ▼
[全局中间件链：RequestID → Metrics → Logger → RedisLimit → Recovery → Cors]
  ▼
router 私有组 → middleware.JWTAuth
  │ ① 校验 Authorization 头（缺失 → 401 请先登录）
  │ ② utils.ParseAccessToken（签名/过期/tokenType=access 校验）
  │       → 失败：401 token无效或登录已过期 / token类型错误
  │ ③ repository.CheckJTIBlacklist(claims.JTI) —— 黑名单 → 401 token已失效
  │ ④ c.Set("userId", claims.UserId)
  ▼
handler.GetUserInfo
  │ 从 context 读取 userId
  ▼
service.GetUserInfo(userID) → repository.GetUserByID(userID)（MySQL 直查，密码置空）
  ▼
response.Success(c, user)
```

### 4.3 AI 请求流程（如 POST /api/ai/generate）

```
客户端
  │ POST /api/ai/generate  Authorization: Bearer <accessToken>  {prompt}
  ▼
[全局中间件链] → JWTAuth 鉴权通过
  ▼
handler.GenerateCode
  │ 绑定 dto.GenerateCodeReq（prompt 必填）
  ▼
service.GenerateCode(prompt) → repository.CallLLM("生成代码：" + prompt)
  │ ai_repository.CallLLM：
  │   ① config 读取 AI_API_KEY / AI_MODEL / AI_URL / AI_TIMEOUT（懒加载；AI_MODEL 未配置时回退 AI_ENDPOINT）
  │   ② 构造 Chat Completions 请求体（system 提示词 + 用户 prompt，temperature=0.3）
  │   ③ context.WithTimeout + http.Client.Timeout（双层超时）
  │   ④ 发起 HTTP POST → 状态码/响应体/JSON 解析逐级错误处理
  │   ⑤ defer metrics.RecordAICall(err == nil) —— 上报 ai_calls_total / ai_failures_total
  ▼
response.Success200(c, 生成文本)   ← 业务码固定 200（兼容 web 前端 data.code === 200 判断）
```

## 5. 路由一览

| 方法 | 路径 | 鉴权 | Handler |
|------|------|------|---------|
| GET | `/metrics` | 无 | `metrics.Handler()` |
| POST | `/api/user/register` | 无 | `handler.Register` |
| POST | `/api/user/login` | 无 | `handler.Login` |
| POST | `/api/user/refresh` | 无 | `handler.RefreshToken` |
| GET | `/api/user/info` | JWT | `handler.GetUserInfo` |
| POST | `/api/user/logout` | JWT | `handler.Logout` |
| PUT | `/api/user/password` | JWT | `handler.UpdatePassword` |
| GET | `/api/user` | JWT | `handler.UserList` |
| PUT | `/api/user/:id` | JWT | `handler.EditUser` |
| DELETE | `/api/user/:id` | JWT | `handler.DeleteUser` |
| PATCH | `/api/user/:id/status` | JWT | `handler.SwitchStatus` |
| POST | `/api/ai/generate` | JWT | `handler.GenerateCode` |
| POST | `/api/ai/explain` | JWT | `handler.ExplainCode` |
| POST | `/api/ai/fix` | JWT | `handler.FixCode` |
| POST | `/api/ai/optimize` | JWT | `handler.OptimizeCode` |

## 6. 与其他文档的关系

- 本文件描述 **当前代码实际状态**（三层架构已全面启用）；
- 仓库根目录 `ARCHITECTURE.md` 为三层架构迁移前的历史审计文档（描述旧 `api/` 扁平架构），仅作演进记录；
- 各设计的深入说明见 [DESIGN.md](DESIGN.md)、接口字段见 [API.md](API.md)。

- AI 提示词构造（`ai_service.go`）。

文件：

| 文件 | 内容 |
|------|------|
| `internal/service/user.go` | 用户全链路业务逻辑 |
| `internal/service/ai_service.go` | AI 四个场景的提示词构造 |

### 2.3 Repository 层（`internal/repository`）

职责：**数据访问**，是唯一允许触碰 `config.DB` / `config.RedisClient` 及发起外部 HTTP 调用的层。

- 用户 CRUD（GORM）；
- Redis 缓存读写与 TTL 管理；
- JTI 黑名单、Refresh Token 双向索引（`rt:jti:*` / `rt:user:*`）与 Lua 原子消费；
- AI Provider 调用（`ai_repository.go` 注释明确：**禁止 handler / service / utils 直接发起 AI HTTP 请求**）。

### 2.4 支撑层

| 层 | 路径 | 职责 |
|----|------|------|
| DTO | `internal/dto` | 请求结构体统一声明（`binding` 校验标签） |
| Model | `model/user.go` | 数据实体（密码字段 `json:"-"` 隐藏） |
| Config | `config/` | 全局 DB / Redis / AI 配置，全部来自环境变量 |
| Router | `router/router.go` | 路由注册与分组中间件 |
| Middleware | `middleware/` | 横切关注点（认证 / 日志 / 限流 / 恢复 / CORS） |
| Utils | `utils/` | JWT 生成解析等无状态工具 |
| Pkg | `pkg/` | 统一响应信封、Prometheus 指标 |

### 2.5 分层依赖规则

```
router → middleware（横切）→ handler → dto（入参）→ service → repository → config / model
                                                              └─→ 第三方（AI API）
```

- 依赖方向单一向下，禁止反向引用（如 repository 调用 handler）；
- handler 不写 SQL、不直接操作 Redis/DB；
- service 不感知 HTTP，不直接访问数据源；
- repository 不处理 HTTP，不承载业务规则。

## 3. 中间件链与执行顺序

注册位置：`cmd/main.go`。

| 顺序 | 中间件 | 文件 | 职责 |
|------|--------|------|------|
| 1 | `RequestID` | `middleware/logger.go` | 生成/透传 `request_id`（必须最外层） |
| 2 | `Metrics` | `pkg/metrics/http.go` | 采集 HTTP 指标（注册在 Recovery 之前，panic 恢复为 500 后仍统计） |
| 3 | `Logger` | `middleware/logger.go` | 请求日志（request_id / ip / method / path / status / latency） |
| 4 | `RedisLimit` | `middleware/redis_limit.go` | Redis 分布式限流（默认 60 次/分钟，超出返回 429） |
| 5 | `Recovery` | `middleware/recovery.go` | panic 恢复（debug 返回细节，release 脱敏） |
| 6 | `Cors` | `middleware/cors.go` | 全项目唯一 CORS 实现（router 中注册） |
| 7 | `JWTAuth` | `middleware/auth.go` | 私有组鉴权（签名 + 过期 + token 类型 + JTI 黑名单） |

> 注：`Cors` 在 `router.InitRouter` 中全局注册，执行顺序仍位于业务 Handler 之前。

### 3.1 请求处理细节

- **RequestID**：优先复用客户端 `X-Request-ID`，否则自动生成 32 位 hex；写入响应头与 gin context；
- **RedisLimit key**：登录用户用 `user:<userId>`，未登录用 IP，拼接接口路径：`limit:<身份>:<route>`；首次计数时设置 1 分钟过期；
- **Recovery**：`gin.Mode() == DebugMode` 返回 `panic: ...` 详情，release 返回 `服务器内部错误`；
- **JWTAuth**：校验 `Authorization: Bearer <token>` → `ParseAccessToken`（独立密钥 + `tokenType == "access"`）→ JTI 黑名单检查 → 写入 `c.Set("userId", ...)`。
