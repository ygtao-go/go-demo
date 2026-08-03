# go-admin 核心设计（DESIGN）

本文档深入说明三大核心设计：**Redis 缓存设计**、**Refresh Token 设计**、**AI 模块设计**。

---

## 1. Redis 缓存设计

### 1.1 连接与请求治理（`config/redis.go`）

| 设计点 | 实现 | 说明 |
|--------|------|------|
| 全局单例 | `config.RedisClient` | goroutine-safe，全项目统一使用 |
| 请求超时 | `RedisContext()` 返回带 3s 超时的 context | 所有 Redis 调用必须经此获取 context，禁止无超时 `context.Background()` |
| Key 环境前缀 | `RedisKey(parts...)` 生成 `<REDIS_PREFIX>:<part>:<part>...` | 多环境 key 隔离；前缀为空时与旧 key 完全兼容 |
| 连接池 | `PoolSize=20`、`MinIdleConns=5`（可配） | 由环境变量控制，默认值可覆盖 |
| fail-fast | 启动 PING 失败直接 `log.Fatalf` | 连接不可用不启动 |
| 生产安全 | `ENV=prod` 强制 Redis 密码非空 | 无密码直接启动失败 |
| 可选 TLS | `REDIS_TLS_ENABLE` / `REDIS_TLS_SKIP_VERIFY` | 默认关闭 |

### 1.2 Key 命名规范

统一经 `config.RedisKey(parts...)` 生成：

```
<prefix>:user:<username>            # 用户基础信息缓存（JSON，不含密码）
<prefix>:user:none:<username>       # “用户不存在”缓存（防穿透）
<prefix>:bl:<jti-hash>              # JTI 黑名单（登出撤销）
<prefix>:rt:jti:<jti-hash>          # refresh token jti → userId（精确撤销/消费）
<prefix>:rt:user:<userId>           # userId → jti-hash 集合（用户维度批量清理）
<prefix>:limit:<user|IP>:<route>    # 分布式限流计数
```

> `<prefix>` 来自 `REDIS_PREFIX`，空则为无前缀，与旧数据兼容。

### 1.3 缓存三防

| 风险 | 方案 | 实现位置 |
|------|------|----------|
| **穿透**（查询不存在的数据） | none 缓存：MySQL 查无记录时写 `user:none:<username>`（TTL 5min），后续请求直接短路 | `repository.SetNoneCachedUser` / `GetNoneCachedUser` |
| **击穿**（热点 key 过期瞬时并发打爆 DB） | singleflight 单飞：同一 username 同时只允许一次“查库 + 回填”，其余并发请求共享结果 | `service.userSingleflight` / `userDBFlight`（`golang.org/x/sync/singleflight`） |
| **雪崩**（大量 key 同时过期） | TTL 随机化：基础 24h ± 1h 抖动，最终落在 23h~25h | `repository.jitteredTTL` |

登录路径与读取路径隔离：

- 读取路径（`GetUserByUsernameProtected`）可命中不含密码的缓存；
- 登录路径（`loadUserFromDB`）走独立的 `userDBFlight`，**必须拿 DB 真实密码哈希** 做 bcrypt 校验，避免并发时误用缓存结果导致登录失败。

### 1.4 缓存内容与一致性

- **缓存不含密码**：`CachedUser` 仅含 ID / Username / Status / 时间戳，刻意省略 Password 字段；
- **写路径主动失效**（Cache-Aside + 主动失效）：

| 写操作 | 失效动作 | 位置 |
|--------|----------|------|
| 注册成功 | 删除残留 `user:none:<username>` | `service.Register` |
| 修改密码 | 删除 `user:<username>` 缓存 | `service.UpdatePassword` |
| 编辑用户 | 删除旧用户名缓存 + 写入新缓存（等价刷新） | `service.EditUser` |
| 删除用户 | 删除用户缓存 + 清理该用户全部 refresh token | `service.DeleteUser` |
| 切换状态 | 删除用户缓存 | `service.SwitchStatus` |

- 缓存写失败不影响主流程（仅记日志），优先保证数据正确性。


---

## 2. Refresh Token 设计

### 2.1 双 Token 结构（`utils/jwt.go`）

| Token | 有效期 | tokenType | 密钥 |
|-------|--------|-----------|------|
| Access Token | 15 分钟 | `access` | `JWT_ACCESS_SECRET` |
| Refresh Token | 7 天 | `refresh` | `JWT_REFRESH_SECRET` |

两者使用**不同密钥 + tokenType 字段**，从 token 本身即可区分类型；`ParseAccessToken` / `ParseRefreshToken` 分别校验对应密钥与类型，Access Token 无法当作 Refresh Token 使用。

### 2.2 Claims

```go
type CustomClaims struct {
    UserId    uint   // 业务用户 ID
    JTI       string // 唯一 Token ID（16 位 hex，随每个 token 生成）
    TokenType string // "access" | "refresh"
    jwt.RegisteredClaims
}
```

- **JTI（JWT ID）** 是撤销与旋转的锚点；Redis 中以 `JTIHash(jti)`（SHA256 前 8 字节 = 16 位 hex）作为 key，缩短 key 长度；
- 密钥从环境变量读取，代码中仅保留默认值兜底，生产必须覆盖。

### 2.3 Refresh Token 的 Redis 持久化（`internal/repository/user.go`）

登录/刷新成功时通过 `SaveRefreshJTI` 写入**双向索引**（Pipeline 原子执行）：

```
SET  rt:jti:<hash> = userId   (TTL 7 天)      # jti → 用户（精确撤销 / 消费）
SADD rt:user:<id>  = <hash>   (TTL 7 天)      # 用户 → jti 集合（批量清理）
```

同步写入（非异步）：确保返回的 refresh token 真实可用，且登出删除后不会被后台任务重新写回。

### 2.4 旋转机制（Rotation）

刷新流程（`service.RefreshToken`）：

```
① utils.ParseRefreshToken（密钥 + 类型 + 过期校验）
② repository.ConsumeRefreshJTI —— Lua 脚本原子“检查 + 删除”：
      if EXISTS rt:jti:<hash> then DEL + SREM return 1 else return 0
      成功 = 本请求独占换新资格；失败 = 已消费/撤销，拒绝（401）
③ 生成新 Access + Refresh（新 JTI）
④ SaveRefreshJTI 持久化新 jti（必须成功，否则视为刷新失败）
```

**并发安全**：Redis 单线程串行执行 Lua 脚本，同一 refresh token 的并发刷新请求**恰好一个成功**（有 `TestConcurrentRefreshOnlyOneWins` 覆盖）。每完成一次刷新，旧 token 立即作废——即 Refresh Token Rotation，可防止重放。

### 2.5 登出与撤销（`service.Logout`）

- Access Token：解析后取其 `ExpiresAt` 剩余有效期，将 JTI 加入黑名单 `bl:<hash>`（TTL=剩余有效期）；
- Refresh Token：解析后 `DeleteRefreshJTI`（同步删除 `rt:jti:*` 与 `rt:user:*` 双向索引），并加入 7 天黑名单兜底；
- 后续请求经 `JWTAuth` 的 `CheckJTIBlacklist` 拦截，返回 `token已失效，请重新登录`。

### 2.6 安全要点汇总

| 要点 | 实现 |
|------|------|
| 双密钥 | access / refresh 使用不同密钥 |
| 类型隔离 | tokenType 声明 + 解析函数分别校验 |
| 短时效 Access | 15 分钟，降低泄漏窗口 |
| Rotation 防重放 | 每次刷新原子消费旧 jti，旧 token 立即失效 |
| 登出即撤销 | JTI 黑名单 + 双向索引删除 |
| 密码不落缓存 | Redis 缓存无 Password 字段，密码校验永远走 DB |

---

## 3. AI 模块设计

### 3.1 分层

AI 模块同样遵循三层架构：

```
handler/ai_handler.go   ← HTTP 输入输出（dto 校验 + Success200）
    │
service/ai_service.go   ← 提示词构造（生成/解释/修复/优化四场景）
    │
repository/ai_repository.go ← AI Provider 唯一入口（HTTP 调用全流程）
    │
    └──► 火山方舟（豆包）Chat Completions API
```

> `ai_repository.go` 文件头注释明确约定：**禁止 handler / service / utils 直接发起 AI HTTP 请求**，AI 外部调用收敛到 repository 单点，便于超时/错误/指标统一治理。

### 3.2 配置（`config/ai.go`）

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `AI_API_KEY` | 无（必填） | 豆包 API Key，生产必须注入，严禁硬编码 |
| `AI_ENDPOINT` | 无 | 火山方舟接入点 ID（作为请求体 `model`） |
| `AI_URL` | `https://ark.cn-beijing.volces.com/api/v3/chat/completions` | API 地址 |
| `AI_TIMEOUT` | 30 | 请求超时（秒） |

配置采用 **`sync.Once` 懒加载 + 幂等**：`InitAI()` 可并发安全多次调用，且在 `main()` 的 `godotenv.Load()` 之后读取，确保本地 `.env` 生效。

### 3.3 调用流程（`CallLLM`）

```
① 读取 AI 配置（APIKey / Endpoint / URL / Timeout）
② 构造 Chat Completions 请求体：
     model = AI_ENDPOINT
     messages = [ {system: 代码学习助手提示词}, {user: prompt} ]
     temperature = 0.3（低温度，代码输出更稳定）
③ json.Marshal → http.NewRequestWithContext（context.WithTimeout）
④ Header：Authorization: Bearer <key>、Content-Type: application/json
⑤ client := &http.Client{Timeout: timeout}   ← 与 context 构成双层超时
⑥ 状态码检查（非 2xx → 报错含响应体）
⑦ 响应体读取 → JSON 解析 → 提取 choices[0].message.content
```

### 3.4 错误处理与超时

- **双层超时**：`context.WithTimeout`（取消请求）+ `http.Client.Timeout`（兜底）；
- 网络超时识别：`net.Error.Timeout()` → 返回 `AI 服务响应超时，请稍后重试`；
- 逐级错误包装：序列化 / 创建请求 / 调用失败 / 状态码异常 / 读响应 / JSON 解析 / 无 choices，均有明确错误信息；
- 超时或失败均返回 error，由 service → handler 透出，HTTP 500。

### 3.5 可观测性

- 每次调用经 `defer metrics.RecordAICall(err == nil)` 上报：
  - `ai_calls_total`（调用总次数）
  - `ai_failures_total`（失败次数，`success=false` 时累加）
- 结合 HTTP 指标（`http_request_duration_seconds`）可观测 AI 接口的 P99 延迟与成功率。

---

## 参考

- 接口定义：[API.md](API.md)
- 架构与请求流程：[ARCHITECTURE.md](ARCHITECTURE.md)
- 监控指标：[MONITORING.md](MONITORING.md)
- 历史审计：`JWT_ANALYSIS.md` / `REDIS_ANALYSIS.md` / `MIGRATION_PLAN.md`

### 1.5 分布式限流（`middleware/redis_limit.go`）

- 算法：固定窗口计数 —— `INCR` 计数，首次（count==1）设置窗口过期时间（默认 60s）；
- 维度：登录用户 `user:<userId>`，未登录 `IP`；再拼接路由 `c.FullPath()`，做到 用户/IP × 接口 细粒度；
- 阈值：`LimitCount = 60` 次 / 分钟，超限返回 HTTP 429；
- 原子性：`INCR` + `Expire` 为 Redis 单命令，天然并发安全；
- 请求级 context + 显式超时（`RedisRequestTimeout`），Redis 异常时返回 `限流服务异常` 而非无限等待。

### 1.6 Key 生命周期一览

| Key | 写入 | 读取 | 过期/清理 |
|-----|------|------|-----------|
| `user:<username>` | CacheUser（24h±1h） | GetCachedUser | 写路径主动 Del |
| `user:none:<username>` | SetNoneCachedUser（5min） | GetNoneCachedUser | 注册/主动 Del |
| `bl:<hash>` | AddJTIBlacklist（剩余有效期） | CheckJTIBlacklist | TTL 自动过期 |
| `rt:jti:<hash>` | SaveRefreshJTI（7 天） | ConsumeRefreshJTI | 消费/登出/清理 |
| `rt:user:<id>` | SaveRefreshJTI（7 天） | CleanUserRefreshTokens | 删除用户时清理 |
