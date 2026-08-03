# JWT 双 Token 架构分析报告

## 一、JWT 生成流程

### 1. 登录成功后在哪里生成 token

```
客户端 POST /api/user/login
    → handler.Login()            [internal/handler/user.go:42]
        → service.Login()        [internal/service/user.go:13]
            → utils.GenerateTokenPair()  [utils/jwt.go:33]
```

生成的 token 通过 `response.Success(c, result)` 返回给客户端，返回结构为：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "accessToken": "eyJhbGciOi...",
    "refreshToken": "eyJhbGciOi..."
  }
}
```

### 2. 调用了哪些函数

| Step | 函数 | 路径 | 职责 |
|------|------|------|------|
| 1 | `service.Login()` | `internal/service/user.go:13` | 验证用户凭证 → 调用 JWT 生成 |
| 2 | `utils.GenerateTokenPair()` | `utils/jwt.go:33` | 生成双 Token（内部调用 `GenerateToken`） |
| 3 | `utils.GenerateToken()` | `utils/jwt.go:18` | 生成单个 JWT Token |
| 4 | `jwt.NewWithClaims()` | golang-jwt 库 | 创建 JWT 签名对象 |
| 5 | `token.SignedString()` | golang-jwt 库 | HS256 算法签名输出 |

### 3. access_token 和 refresh_token 如何生成

`GenerateTokenPair(userId)` 的执行逻辑：

```go
func GenerateTokenPair(userId uint) (accessToken, refreshToken string, err error) {
    // Step 1: 生成 access_token —— 调用 GenerateToken(userId)
    accessToken, err = GenerateToken(userId)
    // GenerateToken 内部使用 Claims{UserId: userId, ExpiresAt: 24h}

    // Step 2: 生成 refresh_token —— 手动构造 Claims
    refreshClaims := Claims{UserId: userId, ExpiresAt: 7天}
    refreshToken, err = 签名(refreshClaims)

    return accessToken, refreshToken, nil
}
```

**本质**：两个 token 是通过**同一个函数逻辑调用两次**生成的，唯一的区别是 `ExpiresAt` 不同：

| Token 类型 | 签名算法 | 密钥 | Claims 结构 | ExpiresAt |
|------------|---------|------|-------------|-----------|
| access_token | HS256 | `var jwtSecret` | `{ userId }` | **24h**（代码写 24h，注释写 2h ❸ 矛盾） |
| refresh_token | HS256 | **同一个** `var jwtSecret` | `{ userId }` | 7 天 |

**⛔ 关键问题**：两者使用**完全相同的密钥 + 完全相同的 Claims 结构**，无法从 token 本身区分类型。

### 4. Claims 包含哪些字段

```go
type Claims struct {
    UserId uint                `json:"userId"`         // 用户 ID
    jwt.RegisteredClaims                                // 内嵌标准声明
}
```

实际编码到 token 中的字段只有：

| 字段 | JSON key | 当前状态 | 说明 |
|------|---------|---------|------|
| `UserId` | `userId` | ✅ 有值 | 唯一业务标识 |
| `ExpiresAt` | `exp` | ✅ 有值 | 过期时间 |
| `IssuedAt` | `iat` | ❌ 缺失 | **无签发时间** |
| `ID` | `jti` | ❌ 缺失 | **无唯一 Token ID** |
| `Issuer` | `iss` | ❌ 缺失 | 可选 |
| `Subject` | `sub` | ❌ 缺失 | 可选 |
| `Audience` | `aud` | ❌ 缺失 | 可选 |
| `NotBefore` | `nbf` | ❌ 缺失 | 可选 |

**缺失 `iat`、`jti`、`token_type` 三个关键字段**：
- 缺少 `iat` → 无法计算 token 已存在时间
- 缺少 `jti` → 无法做 Token 指纹追踪、无法精确撤销单个 token
- 缺少 `token_type`（自定义）→ 无法从 token 自身区分 access 还是 refresh

### 5. token 有效期

| Token | 注释声明 | 实际代码硬编码 | 企业实践建议 |
|-------|---------|---------------|-------------|
| access_token | "2小时过期" | **24h**（`24 * time.Hour`） | 15~30 分钟 |
| refresh_token | "7天过期" | 7 天（`7 * 24 * time.Hour`） | 7~30 天 |

**⛔ access_token 的注释与实际代码严重矛盾**：第 35 行注释写 `// Access Token：2小时过期`，但调用的 `GenerateToken()` 内部实际是 `24 * time.Hour`（第 19 行）。这是一个**危险的误导**，后续维护者如果只看注释会误以为 access_token 只有 2h 有效期。

---

## 二、JWT 解析流程

### 1. 请求携带 token 后经过哪些组件

```
客户端请求（携带 Authorization: Bearer <token>）
    ↓
Cors() 中间件                    [router/router.go:51]   跨域处理
    ↓
JWTAuth() 中间件                 [middleware/auth.go:12] 鉴权
    ↓
业务 Handler                     [internal/handler/]     处理请求
```

### 2. middleware/auth.go 如何解析 token

```go
func JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Phase 1: 提取 token
        authHeader := c.GetHeader("Authorization")
        // 校验格式 "Bearer <token>"
        tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

        // Phase 2: 黑名单检查（Redis）
        if blocked, _ := repository.CheckBlacklist(tokenStr); blocked {
            // token 已被撤销 → 拒绝
            c.Abort()
            return
        }

        // Phase 3: JWT 解析验证
        claims, err := utils.ParseToken(tokenStr)
        // ParseToken 内部：
        //   - jwt.ParseWithClaims(tokenStr, &Claims{}, keyFunc)
        //   - keyFunc 返回 jwtSecret
        //   - 验证签名 + 过期时间
        if err != nil {
            // token 无效或过期 → 拒绝
            c.Abort()
            return
        }

        // Phase 4: 注入上下文
        c.Set("userId", claims.UserId)
        c.Next()
    }
}
```

### 3. 如何获取 userId

```go
// 中间件中注入
c.Set("userId", claims.UserId)  // claims.UserId 是 uint 类型

// 业务 Handler 中取出
userId, exists := c.Get("userId")
// userId 类型为 interface{}，需要断言为 uint
user, err := service.GetUserInfo(userId.(uint))
```

### 4. token 验证失败有哪些情况

| 失败原因 | 触发条件 | 客户端看到的消息 |
|---------|---------|----------------|
| 无 token | 请求头无 `Authorization` 或格式不是 `Bearer ` | "请先登录" |
| token 被撤销 | Redis 黑名单中存在该 token | "token已失效，请重新登录" |
| token 过期 | `exp` 字段时间已过 | "token无效或登录已过期" |
| 签名无效 | token 被篡改或密钥不匹配 | "token无效或登录已过期" |
| 格式错误 | token 字符串不是合法 JWT 格式 | "token无效或登录已过期" |

**⛔ 注意**：第 2、3、4、5 种情况返回的是**相同的错误消息**，无法区分是 token 过期还是被篡改，对调试不友好。

---

## 三、JWT 生命周期

```
┌──────────────────────────────────────────────────────────────────┐
│  【登录阶段】                                                     │
│                                                                  │
│  客户端 → POST /api/user/login                                  │
│       ↓                                                         │
│  handler.Login()                                                │
│       ↓                                                         │
│  service.Login()                                                │
│       ├─ 查用户（缓存 → DB）                                     │
│       ├─ 验证密码 (bcrypt)                                       │
│       ├─ 异步缓存用户 → Redis                                    │
│       └─ GenerateTokenPair(userId)                              │
│            ├─ GenerateToken(userId)    → access_token (24h)     │
│            └─ 手动构造 refresh Claims → refresh_token (7天)     │
│       ↓                                                         │
│  返回 { accessToken, refreshToken } 给客户端                      │
│                                                                  │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │
│  【使用阶段】                                                     │
│                                                                  │
│  客户端保存双 Token（localStorage / sessionStorage）              │
│       ↓                                                         │
│  请求业务接口 Authorization: Bearer <access_token>               │
│       ↓                                                         │
│  Cors() 中间件（跨域）                                           │
│       ↓                                                         │
│  JWTAuth() 中间件                                                │
│       ├─ 提取 Bearer token                                      │
│       ├─ Redis 检查黑名单                                        │
│       ├─ JWT 解析验证签名 + 过期                                │
│       └─ c.Set("userId", claims.UserId)                         │
│       ↓                                                         │
│  业务 Handler 处理                                               │
│       ↓                                                         │
│  返回响应给客户端                                                │
│                                                                  │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │
│  【撤销阶段】                                                     │
│                                                                  │
│  POST /api/user/logout                                          │
│       ↓                                                         │
│  handler.Logout()                                               │
│       ↓                                                         │
│  service.Logout(token)                                          │
│       └─ AddToBlacklist(token, 24h)                             │
│            └─ Redis SET "blacklist:<完整token>" "1" EX 86400    │
│                                                                  │
│  ⛔ 注意：只撤销了 access_token                                 │
│  ⛔ refresh_token 没有任何撤销                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## 四、当前代码问题

### 问题 1：access_token 与 refresh_token 未有效区分

| 维度 | 当前实现 | 问题 |
|------|---------|------|
| Claims 结构 | 两者完全一样（只有 `UserId` + `exp`） | **无法从 token 自身判断是 access 还是 refresh** |
| 签名密钥 | 共用 `var jwtSecret` | 密钥泄露后两个 token 同时失效，无法隔离风险 |
| 生成函数 | `GenerateTokenPair` 内部复用了 `GenerateToken` | access 和 refresh 的 Claims 构造路径不一致，有一份注释误导 |

**风险**：如果把 refresh_token 传给业务接口的 `Authorization` 头，中间件照样能解析通过，因为没有 `token_type` 校验。

### 问题 2：secret 硬编码

```go
var jwtSecret = []byte("GoAdmin2026SecretKey")
```

- **硬编码在源码中**，无法按环境（dev/test/prod）切换
- **所有开发者共享同一个密钥**，无法追溯谁签发了哪个 token
- **密钥轮换**（secret rotation）需要修改源码重新部署
- Git 提交后密钥进入版本历史，长期暴露

### 问题 3：Claims 设计不完整

**当前 Claims**：
```go
type Claims struct {
    UserId uint
    jwt.RegisteredClaims  // 只用了 ExpiresAt
}
```

**缺少的标准字段**：

| 缺少字段 | 用途 | 影响 |
|---------|------|------|
| `jti` (JWT ID) | 唯一标识每个 token | 无法做精确撤销、无法追踪 token 生命周期 |
| `iat` (Issued At) | 签发时间 | 无法判断 token 已使用多久 |
| `token_type` (自定义) | 区分 access/refresh | 无法防止 token 类型混淆攻击 |

### 问题 4：token 安全问题

| 安全问题 | 严重程度 | 细节 |
|---------|---------|------|
| access_token 24h 太长 | 🔴 高 | 泄露后攻击窗口过大，标准应为 15-30 分钟 |
| 无 refresh 端点 | 🔴 高 | refresh_token 发了但没有接口能用，用户只能重新登录 |
| Logout 只撤销 access_token | 🔴 高 | refresh_token 仍然有效 7 天，可被继续使用 |
| 黑名单 key 用完整 token 字符串 | 🟠 中 | token 约 150+ 字符，浪费 Redis 内存，应该用哈希摘要 |

### 问题 5：Redis 黑名单不完整

```go
func Logout(token string) error {
    return repository.AddToBlacklist(token, 24*time.Hour)
}
```

- **只传入了 access_token**（从 `Authorization` 头提取）
- **refresh_token 无法被撤销**，客户端没有提交 refresh_token 的机制
- **TTL 写死 24h**，没有根据 token 的剩余有效期动态计算
- **黑名单 key 过长**：`"blacklist:" + token` 中 token 是完整的 JWT 字符串

---

## 五、改造建议

### P0 — 必须修复（安全阻塞）

| # | 问题 | 方案 |
|---|------|------|
| 0.1 | access_token 有效期 24h 过长 | 改为 **15 分钟**（`15 * time.Minute`），同步修正注释 |
| 0.2 | 没有 refresh 端点 | 新增 `POST /api/user/refresh`：验证 refresh_token → 检查 Redis jti → 生成新双 Token → 旧 refresh 失效 |
| 0.3 | Logout 只撤销 access_token | 要求客户端在请求体中同时提交 refresh_token，两个一起加入黑名单 |
| 0.4 | 密钥硬编码 | 改为从环境变量 `JWT_ACCESS_SECRET` 和 `JWT_REFRESH_SECRET` 读取，部署时注入 |

### P1 — 建议优化

| # | 问题 | 方案 |
|---|------|------|
| 1.1 | Claims 缺少 jti、iat、token_type | 增加三个字段：`jti`（uuid）、`iat`（time.Now()）、`token_type`（"access"/"refresh"） |
| 1.2 | access 和 refresh 用同一密钥 | 拆分为两个独立密钥：`jwtAccessSecret` 和 `jwtRefreshSecret`，互不影响 |
| 1.3 | 黑名单用完整 token 做 key | 改为 `sha256(token)[:16]` 作为 Redis key，长度固定为 32 字符 |
| 1.4 | 黑名单 TTL 写死 24h | 动态计算：从 `claims.ExpiresAt - time.Now()` 得到剩余时间，设为黑名单 TTL |
| 1.5 | JWTAuth 无法区分 token 类型 | 解析成功后检查 `claims.token_type`，如果是 refresh_token 拒绝（不允许用 refresh 调业务接口） |
| 1.6 | 错误消息过于笼统 | 区分"token 过期"（401）、"token 被撤销"（401）、"签名无效"（401），返回不同错误码便于前端处理 |

### P2 — 长期优化

| # | 问题 | 方案 |
|---|------|------|
| 2.1 | Claims 中无其他标识 | 增加 `sub` 字段 = fmt.Sprintf("user:%d", userId)，遵循标准 |
| 2.2 | 无法做密钥轮换 | 支持多密钥验证：旧密钥签发的 token 在过渡期内仍然有效，新 token 使用新密钥 |
| 2.3 | 无 Token 生命周期追踪 | `jti` + Redis 记录 token 签发时间、客户端 IP、User-Agent，支持审计 | 2.4 | 所有 Token 逻辑集中在 service 层 | 将 JWT 双 Token 生成、刷新、撤销封装为 `internal/service/auth_service.go`，与 `user_service.go` 解耦 |

---

## 附录：改造前后对照

| 维度 | 改造前 | 改造后 |
|------|-------|-------|
| Claims 字段 | `{ userId, exp }` | `{ userId, token_type, jti, iat, exp, sub }` |
| 密钥 | 1 个硬编码密钥 | 2 个环境变量密钥 |
| access_token 有效期 | 24h | 15min |
| refresh 端点 | ❌ 不存在 | ✅ `POST /api/user/refresh` |
| token 类型验证 | ❌ 无法区分 | ✅ 中间件校验 token_type |
| 黑名单 key | 完整 token 字符串 | SHA256 摘要 |
| 黑名单 TTL | 写死 24h | 动态计算剩余时间 |
| Logout 撤销范围 | 仅 access_token | access + refresh 双撤销 |
| 密钥环境变量 | ❌ 硬编码 | ✅ `os.Getenv("JWT_ACCESS_SECRET")` |