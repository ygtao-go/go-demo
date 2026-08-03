# JWT 双 Token 改造验证报告

> 验证时间：2026/7/27 16:36
> 验证目标：确认 utils/jwt.go、middleware/auth.go、internal/service/user.go、router/router.go 是否按设计要求完成改造

---

## 1️⃣ utils/jwt.go 验证

### 1.1 是否存在 `GenerateAccessToken`？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| `GenerateAccessToken` 函数 | ❌ **不存在** | 只有 `GenerateToken`（通用单 Token）和 `GenerateTokenPair`（双 Token 生成器） |
| 当前实际生成 access_token 的方式 | `GenerateToken(userId)` → 24h 过期 | 调用的是通用函数，无法设置不同 TTL |

**问题**：没有独立的 access token 生成函数，当前 `GenerateToken` 是统一函数，access 和 refresh 生成路径不一致。

### 1.2 是否存在 `GenerateRefreshToken`？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| `GenerateRefreshToken` 函数 | ❌ **不存在** | refresh token 在 `GenerateTokenPair` 内部手动构造 Claims 生成，无独立函数 |

**问题**：refresh token 生成逻辑嵌入在 `GenerateTokenPair` 内，无法独立调用（如 refresh 端点需要生成新的 refresh token 时无法复用）。

### 1.3 access/refresh 是否使用不同 secret？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 密钥数量 | ❌ **仅 1 个** | `var jwtSecret = []byte("GoAdmin2026SecretKey")` |
| 密钥隔离 | ❌ **未隔离** | access 和 refresh 共用 `jwtSecret` |
| 密钥读取方式 | ❌ **硬编码** | 写在代码中，不是从环境变量读取 |

**问题**：单一密钥、硬编码、无环境变量注入。密钥泄露后两个 token 全部失效。

### 1.4 Claims 是否包含全部必须字段？

| 字段 | 要求 | 实际状态 | 结果 |
|------|------|---------|------|
| `userId` | ✅ 必须 | `UserId uint \`json:"userId"\`` | ✅ **存在** |
| `jti` (JWT ID) | ✅ 必须 | ❌ 未定义 | ❌ **缺失** |
| `iat` (Issued At) | ✅ 必须 | ❌ `RegisteredClaims` 中未设置 | ❌ **缺失** |
| `exp` (Expires At) | ✅ 必须 | `jwt.RegisteredClaims.ExpiresAt` | ✅ **存在** |
| `token_type` | ✅ 必须 | ❌ 未定义 | ❌ **缺失** |

**结论**：Claims 仅 2/5 达标，缺少 `jti`、`iat`、`token_type` 三个关键字段。

#### Claims 定义对比

```go
// 改造前（当前代码）
type Claims struct {
    UserId uint                `json:"userId"`
    jwt.RegisteredClaims       // 只用了 ExpiresAt
}

// 改造后（要求）
type Claims struct {
    UserId    uint   `json:"userId"`
    TokenType string `json:"token_type"` // 新增
    jwt.RegisteredClaims                  // 需要设置 jti, iat, exp
}
```

---

## 2️⃣ middleware/auth.go 验证

### 2.1 是否验证 access_token 类型？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 解析后检查 `token_type` | ❌ **未验证** | 中间件解析 Claims 后直接取 `claims.UserId`，未检查 `token_type` |
| 拒绝 refresh_token 访问业务接口 | ❌ **未实现** | 将 refresh_token 放入 `Authorization` 头可以正常通过 |

**风险**：refresh_token 可以被用来调用业务接口，存在 token 类型混淆安全问题。

### 2.2 是否检查 Redis blacklist？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 黑名单检查 | ✅ **已实现** | 第 30 行：`repository.CheckBlacklist(tokenStr)` |
| 检查时机 | ✅ 合理 | 在 JWT 解析前检查，减少无效解析 |
| 黑名单 key 长度 | ❌ **过长** | `"blacklist:" + token`，完整 JWT 约 150+ 字符 |
| 黑名单 TTL | ❌ **写死 24h** | 不是根据 token 剩余有效期动态计算 |

**结论**：黑名单功能存在但实现粗糙，key 过长、TTL 不精确。

---

## 3️⃣ internal/service/user.go 验证

### 3.1 Login 是否返回 accessToken + refreshToken？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| `accessToken` 字段 | ✅ **已返回** | `map["accessToken"] = accessToken` |
| `refreshToken` 字段 | ✅ **已返回** | `map["refreshToken"] = refreshToken` |
| refresh_token 是否存储到 Redis | ❌ **未存储** | 生成后直接返回给客户端，未持久化 |

**问题**：refresh_token 生成后未持久化到 Redis，服务端无法主动撤销特定 refresh_token。

### 3.2 Logout 是否同时撤销两个 token？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 撤销 access_token | ✅ **已撤销** | 从 `Authorization` 头提取并加入黑名单 |
| 撤销 refresh_token | ❌ **未撤销** | handler 未接收 refresh_token，service 未处理 |
| 同时撤销双 token 的逻辑 | ❌ **不存在** | `service.Logout(token)` 只接收一个 token 参数 |

**问题**：Logout 后 refresh_token 仍然有效 7 天。

#### 当前 Logout 调用链

```
handler.Logout(c)
  ├─ 从 c.GetHeader("Authorization") 提取 token  ← 只有 access_token
  └─ service.Logout(token)                       ← 只撤销这一个
       └─ repository.AddToBlacklist(token, 24h)  ← 只加这一个
```

#### 改造后要求

```
handler.Logout(c)
  ├─ 从 Header 提取 access_token
  ├─ 从请求体提取 refresh_token
  └─ service.Logout(accessToken, refreshToken)
       ├─ AddToBlacklist(accessToken, ttl)
       └─ AddToBlacklist(refreshToken, ttl)
       └─ Redis DEL "refresh_jti:<jti>"  ← 删除 refresh 记录
```

---

## 4️⃣ router/router.go 验证

### 4.1 是否存在 `POST /api/user/refresh`？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| `/api/user/refresh` 路由 | ❌ **不存在** | 路由中没有任何 refresh 相关条目 |
| 任何 refresh 相关端点 | ❌ **不存在** | 公共组和私有组均未定义 |

**问题**：这是双 Token 体系的核心痛点。refresh_token 在登录时返回给客户端，但没有任何接口能消费它，access_token 过期后用户只能重新登录。

---

## 5️⃣ refresh_token 机制验证

### 5.1 是否保存到 Redis？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| refresh_token 持久化存储 | ❌ **未存储** | `GenerateTokenPair` 只生成和返回，不写入任何存储 |
| Redis 中 refresh 相关 key | ❌ **不存在** | 没有任何以 `refresh` 开头的 Redis key |

**问题**：服务端对已签发的 refresh_token 完全无状态、无追踪能力。

### 5.2 是否支持 rotation（轮换）？

| 检查项 | 结果 | 说明 |
|--------|------|------|
| refresh 时生成新 refresh_token | ❌ **不适用** | refresh 端点不存在 |
| 旧 refresh_token 立即失效 | ❌ **不适用** | refresh 端点不存在 |
| refresh 时更新 Redis 记录 | ❌ **不适用** | refresh 端点不存在 |

**结论**：rotation 机制完全未实现，这是双 Token 体系中防止 refresh_token 泄露的核心安全机制。

---

## 6️⃣ 总体验证汇总

| 验证项目 | 要求 | 实际 | 状态 |
|---------|------|------|------|
| **utils/jwt.go** | | | |
| GenerateAccessToken | ✅ 存在独立函数 | ❌ 只有通用 `GenerateToken` | ❌ |
| GenerateRefreshToken | ✅ 存在独立函数 | ❌ 嵌入在 `GenerateTokenPair` 内 | ❌ |
| 不同 secret | ✅ access/refresh 使用不同密钥 | ❌ 共用 `jwtSecret` | ❌ |
| Claims.userId | ✅ 存在 | ✅ 存在 | ✅ |
| Claims.jti | ✅ 存在 | ❌ 缺失 | ❌ |
| Claims.iat | ✅ 存在 | ❌ 缺失 | ❌ |
| Claims.exp | ✅ 存在 | ✅ 存在 | ✅ |
| Claims.token_type | ✅ 存在 | ❌ 缺失 | ❌ |
| secret 非硬编码 | ✅ 从环境变量读取 | ❌ 硬编码在源码 | ❌ |
| **middleware/auth.go** | | | |
| 验证 token_type | ✅ 拒绝 refresh_token 访问业务接口 | ❌ 未校验 | ❌ |
| 检查 Redis blacklist | ✅ 已实现 | ✅ 已实现 | ✅ |
| 黑名单 key 使用 hash | ✅ SHA256 摘要缩短 key | ❌ 完整 token 字符串 | ❌ |
| **service/user.go** | | | |
| Login 返回 accessToken | ✅ 已返回 | ✅ 已返回 | ✅ |
| Login 返回 refreshToken | ✅ 已返回 | ✅ 已返回 | ✅ |
| refresh_token 持久化 | ✅ 存储到 Redis | ❌ 未存储 | ❌ |
| Logout 撤销双 token | ✅ access + refresh 同时注销 | ❌ 只撤销 access_token | ❌ |
| **router/router.go** | | | |
| POST /api/user/refresh | ✅ 存在 refresh 端点 | ❌ 不存在 | ❌ |
| **refresh 机制** | | | |
| Redis 保存 refresh jti | ✅ 已存储 | ❌ 未存储 | ❌ |
| 支持 rotation | ✅ 刷新时轮换 token | ❌ 未实现 | ❌ |

### 统计

| 级别 | 通过 | 不通过 | 通过率 |
|------|------|--------|--------|
| ✅ 已实现 | 6 项 | — | — |
| ❌ 未实现 | — | 14 项 | — |
| **总计** | **6** | **14** | **30%** |

---

## 7️⃣ 未通过项按优先级排序

| 优先级 | 未实现项 | 所属文件 | 影响 |
|--------|---------|---------|------|
| 🔴 P0 | 无 refresh 端点 | router/router.go | refresh_token 无法使用，双 Token 名存实亡 |
| 🔴 P0 | access_token 24h 过长 | utils/jwt.go | 泄露后攻击窗口过大 |
| 🔴 P0 | Logout 只撤销 access_token | service/user.go | refresh_token 仍然有效 7 天 |
| 🔴 P0 | Claims 缺少 jti | utils/jwt.go | 无法做 token 指纹追踪和精确撤销 |
| 🔴 P0 | Claims 缺少 iat | utils/jwt.go | 无法计算 token 使用时长 |
| 🔴 P0 | Claims 缺少 token_type | utils/jwt.go | 无法区分 access/refresh，存在类型混淆攻击风险 |
| 🔴 P0 | 密钥硬编码 | utils/jwt.go | 无法按环境切换，泄露后全量 token 失效 |
| 🔴 P0 | access/refresh 共用密钥 | utils/jwt.go | 密钥隔离缺失 |
| 🟠 P1 | refresh_token 未持久化 | service/user.go + repository | 无法主动撤销特定 refresh_token |
| 🟠 P1 | 无 GenerateAccessToken 独立函数 | utils/jwt.go | 代码复用性差，不易扩展 |
| 🟠 P1 | 无 GenerateRefreshToken 独立函数 | utils/jwt.go | refresh 端点无法独立调用生成逻辑 |
| 🟠 P1 | 黑名单 key 用完整 token | middleware/auth.go | Redis 内存浪费 |
| 🟠 P1 | 黑名单 TTL 写死 24h | service/user.go | 应动态计算剩余有效期 |
| 🟡 P2 | 中间件未校验 token_type | middleware/auth.go | 安全防护不完整 |

---

## 8️⃣ 结论

**本次 JWT 双 Token 改造未完成。**

代码仍然处于**改造前的原始状态**，所有要求的改造项目均未实施：

1. **JWT 结构未增强**：Claims 还是只有 `{ userId, exp }`，新增的 `jti`、`iat`、`token_type` 全部缺失
2. **密钥体系未拆分**：access/refresh 仍然共用 1 个硬编码密钥
3. **refresh 端点不存在**：`POST /api/user/refresh` 路由未添加
4. **Logout 未增强**：仍然只接收 access_token，不处理 refresh_token
5. **refresh_token 未持久化**：生成即返回，无 Redis 存储，无 rotation

**下一步建议**：按三轮改造计划从 `utils/jwt.go` JWT 结构改造开始执行。