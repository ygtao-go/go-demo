# go-admin 接口文档（API）

## 1. 通用说明

| 项 | 说明 |
|----|------|
| Base URL | `http://localhost:8080` |
| 数据格式 | `application/json` |
| 鉴权方式 | `Authorization: Bearer <accessToken>`（仅需鉴权接口） |
| 统一响应信封 | `{"code": <int>, "msg": <string>, "data": <any|null>}` |

**响应信封约定**：

- `code = 0` 表示业务成功（AI 模块为 `code = 200`，兼容 web 前端 `data.code === 200` 判断）；
- 失败时 `code` 与 HTTP 状态码一致（如 400 / 401 / 404 / 429 / 500），`msg` 为错误描述，`data` 为 `null`。

| HTTP 状态码 | 业务码 | 常见场景 |
|-------------|--------|----------|
| 200 | 0（AI：200） | 成功 |
| 400 | 400 | 参数错误 / 用户名已存在 / 密码错误 / 旧密码错误 |
| 401 | 401 | 未登录 / token 无效 / token 已失效 / refresh 已消费 |
| 404 | 404 | 用户不存在 / 目标资源不存在 |
| 429 | 429 | 请求过于频繁（限流） |
| 500 | 500 | 服务器内部错误 / AI 服务异常 |

## 2. 公开接口（无需鉴权）

### 2.1 用户注册

```
POST /api/user/register
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 是 | 用户名（2~20 位） |
| `password` | string | 是 | 密码（6~20 位） |

响应示例：

```json
{ "code": 0, "msg": "success", "data": "注册成功" }
```

### 2.2 用户登录

```
POST /api/user/login
```

请求体：`username` / `password`（同上）。

响应示例：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "accessToken": "eyJhbGciOi...",
    "refreshToken": "eyJhbGciOi...",
    "accessJTI": "a1b2c3d4e5f6a7b8",
    "refreshJTI": "9f8e7d6c5b4a3921"
  }
}
```

### 2.3 刷新 Token

```
POST /api/user/refresh
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `refreshToken` | string | 是 | Refresh Token（**Access Token 不可用**，返回 401） |

刷新采用 **Rotation 机制**：旧 refresh token 原子消费作废，返回全新的双 Token（结构同登录响应）。并发刷新同一 refresh token 时仅一个成功。

### 2.4 Prometheus 指标端点

```
GET /metrics
```

Prometheus 文本格式指标，无需鉴权。指标列表见 [MONITORING.md](MONITORING.md)。

## 3. 需鉴权接口（`Authorization: Bearer <accessToken>`）

### 3.1 获取用户信息

```
GET /api/user/info
```

响应：

```json
{
  "code": 0, "msg": "success",
  "data": {
    "id": 1, "username": "admin", "status": 1,
    "created_at": "2026-08-03T10:00:00+08:00",
    "updated_at": "2026-08-03T10:00:00+08:00"
  }
}
```

> `password` 字段不会出现在任何响应中（model `json:"-"` + 查询后置空）。

### 3.2 退出登录

```
POST /api/user/logout
```

请求体：`refreshToken`（string，必填）。Access Token 从请求头取。

撤销逻辑：access JTI 加入黑名单（TTL=剩余有效期）；refresh 双向索引删除并加入 7 天黑名单。响应：`{"code":0,"msg":"success","data":"退出成功"}`。

### 3.3 修改密码

```
PUT /api/user/password
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `oldPassword` | string | 是 | 旧密码 |
| `newPassword` | string | 是 | 新密码 |

成功后删除用户缓存。响应：`{"code":0,"msg":"success","data":"修改成功"}`。

### 3.4 用户列表

```
GET /api/user?page=1&pageSize=10
```

Query 参数：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码（≤0 时按 1） |
| `pageSize` | int | 10 | 每页条数（≤0 或 >100 时按 10） |

响应：

```json
{
  "code": 0, "msg": "success",
  "data": { "list": [ { "id":1, "username":"admin", "status":1 } ], "total": 1 }
}
```

### 3.5 编辑用户

```
PUT /api/user/:id
```

路径参数：`id`（用户 ID）。请求体：`username`（string，可选）、`status`（int，可选，非 0 才更新）。

> 改名的同时会删除旧用户名缓存并写入新缓存。响应：`{"code":0,"msg":"success","data":"更新成功"}`。

### 3.6 删除用户

```
DELETE /api/user/:id
```

删除用户数据 + 用户缓存 + 该用户全部 refresh token 索引。响应：`{"code":0,"msg":"success","data":"删除成功"}`。

### 3.7 切换用户状态

```
PATCH /api/user/:id/status
```

请求体：`status`（int，必填，`1=正常` / `2=禁用`，其它值返回 400）。

响应：`{"code":0,"msg":"success","data":"状态更新成功"}`。

### 3.8 AI 代码生成

```
POST /api/ai/generate
```

请求体：`prompt`（string，必填，代码生成需求描述）。

响应（业务码 200）：

```json
{ "code": 200, "msg": "success", "data": "生成的代码文本" }
```

### 3.9 AI 代码解释

```
POST /api/ai/explain
```

请求体：`code`（string，必填，待解释代码）。响应结构同 3.8。

### 3.10 AI 代码修复

```
POST /api/ai/fix
```

请求体：`code`（string，必填）。响应结构同 3.8。

### 3.11 AI 代码优化

```
POST /api/ai/optimize
```

请求体：`code`（string，必填）。响应结构同 3.8。

### 3.12 Dashboard 数据看板统计

```
GET /api/dashboard/statistics
```

无需请求体。响应 `data` 为 `DashboardStatistics`（字段类型与 `go-admin/internal/dto/dashboard.go` 完全一致）：

| 字段 | 类型 | 说明 | 数据来源 |
|------|------|------|----------|
| `userCount` | number | 用户数量 | MySQL `users` 表 `COUNT(*)` |
| `aiCallCount` | number | AI 调用次数（成功） | Redis `dashboard:ai_calls` |
| `aiErrorCount` | number | AI 调用失败次数 | Redis `dashboard:ai_errors` |
| `requestCount` | number | 接口请求次数 | Redis `dashboard:http_requests` |
| `errorCount` | number | 接口错误次数（status >= 400） | Redis `dashboard:http_errors` |

计数写入说明（**保留原有 Prometheus 指标，额外同步 Redis**）：

- AI 调用：`repository.CallLLM` 成功 → `dashboard:ai_calls +1`；失败 → `dashboard:ai_errors +1`；
- HTTP 请求：`middleware.Logger` 每请求 → `dashboard:http_requests +1`，`status >= 400` 时额外 `dashboard:http_errors +1`（`/metrics` 自身不计入）。

响应示例：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "userCount": 0,
    "aiCallCount": 0,
    "aiErrorCount": 0,
    "requestCount": 0,
    "errorCount": 0
  }
}
```

## 4. 请求 DTO 字段汇总

| DTO | 字段 | 校验 |
|-----|------|------|
| `LoginReq` / `RegisterReq` | `username`、`password` | 均 `required` |
| `RefreshReq` / `LogoutReq` | `refreshToken` | `required` |
| `UpdatePasswordReq` | `oldPassword`、`newPassword` | 均 `required` |
| `EditUserReq` | `username`、`status` | 可选 |
| `SwitchStatusReq` | `status` | `required` |
| `PageReq` | `page`、`pageSize` | Query 参数 |
| `GenerateCodeReq` | `prompt` | `required` |
| `CodeReq`（解释/修复/优化共用） | `code` | `required` |

## 5. 鉴权失败场景

| 场景 | HTTP | msg |
|------|------|-----|
| 未携带 Authorization | 401 | 请先登录 |
| token 无效 / 过期 | 401 | token无效或登录已过期 |
| 用 refresh token 访问受保护接口 | 401 | token类型错误，请使用access_token |
| JTI 在黑名单（已登出） | 401 | token已失效，请重新登录 |
| 用 access token 刷新 | 401 | refresh token无效 |
| refresh token 已被消费/撤销 | 401 | refresh token已过期或已被撤销 |

## 6. 调用示例（curl）

```bash
BASE=http://localhost:8080

# 1) 注册
curl -s -X POST $BASE/api/user/register -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"123456"}'

# 2) 登录
TOKEN=$(curl -s -X POST $BASE/api/user/login -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"123456"}' | jq -r '.data.accessToken')

# 3) 获取用户信息
curl -s $BASE/api/user/info -H "Authorization: Bearer $TOKEN"

# 4) 刷新 token
curl -s -X POST $BASE/api/user/refresh -H 'Content-Type: application/json' \
  -d "{\"refreshToken\":\"$REFRESH_TOKEN\"}"

# 5) AI 代码解释（需要 AI_API_KEY / AI_ENDPOINT 已配置）
curl -s -X POST $BASE/api/ai/explain -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{"code":"fmt.Println(\"hi\")"}'

# 6) Dashboard 数据看板统计
curl -s $BASE/api/dashboard/statistics -H "Authorization: Bearer $TOKEN"
```
