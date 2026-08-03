# go-admin 三层架构迁移计划

## 架构总纲：分层职责

```
┌──────────────────────────────────────────────────────────────┐
│  Handler 层 (internal/handler/)                              │
│  职责：仅处理 HTTP 输入输出                                   │
│  · 请求绑定 (ShouldBindJSON / ShouldBindQuery)               │
│  · 参数校验 (binding tag 校验 + 自定义校验)                   │
│  · 提取上下文 (c.Get("userId") / c.Param("id"))              │
│  · 调用 Service 层                                            │
│  · 统一返回响应 (response.Success / response.Fail)            │
│  禁止：直接操作 config.DB / config.RedisClient               │
│  禁止：做业务判断 (密码校验、Token生成、规则判断)              │
└──────────────────┬───────────────────────────────────────────┘
                   │ 调用
                   ▼
┌──────────────────────────────────────────────────────────────┐
│  Service 层 (internal/service/)                              │
│  职责：承载全部业务逻辑                                       │
│  · 参数二次校验 (用户名唯一性、密码复杂度等)                   │
│  · 密码加密/比对 (bcrypt)                                    │
│  · Token 生成/刷新 (JWT)                                     │
│  · 缓存策略 (触发 CacheUser、黑名单等异步逻辑)                │
│  · 调用 Repository 层                                         │
│  禁止：接收 gin.Context                                     │
│  禁止：直接操作 config.DB / config.RedisClient               │
└──────────────────┬───────────────────────────────────────────┘
                   │ 调用
                   ▼
┌──────────────────────────────────────────────────────────────┐
│  Repository 层 (internal/repository/)                        │
│  职责：只做数据存取，不含业务逻辑                              │
│  · MySQL 查询 (CRUD)                                         │
│  · Redis 读写 (缓存、黑名单等)                                │
│  · 操作 config.DB / config.RedisClient (唯一授权层)           │
│  禁止：做密码校验、Token 生成、业务判断                       │
└──────────────────────────────────────────────────────────────┘
```

## 接口迁移清单

### 第一阶段：用户模块（最高优先级）

#### Step 1：完善登录链路（已有基础，优先打通）

| 层 | 文件 | 需要做什么 |
|----|------|-----------|
| Repository | `internal/repository/user.go` | ✅ `GetUserByUsername` 已存在；✅ `CacheUser` 已存在；需新增 `CheckBlacklist`、`AddToBlacklist` |
| Service | `internal/service/user.go` | 升级：废弃旧伪 Token，接入标准 JWT 双 Token（access + refresh）；继承现有 Login 逻辑 |
| Handler | `internal/handler/user.go` | `Login` 已存在，检查是否适配新接口 |
| Router | `router/router.go` | 将 `POST /api/user/login` 从 `api.Login` 改为 `handler.Login` |
| **测试** | — | 登录返回标准 JWT → 鉴权中间件正常解析 → 全链路通过 |
| **清理** | `api/user.go` | ✅ 测试通过后，**立即删除 `Login` 函数** |

#### Step 2：依次迁移剩余用户接口（按顺序）

##### 2.1 注册 `/api/user/register`

| 层 | 文件 | 方法 | 说明 |
|----|------|------|------|
| Repository | `internal/repository/user.go` | `CreateUser(user *model.User) error` | insert 用户记录 |
| | | `GetUserByUsername(username string) (*model.User, error)` | 已存在，检查用户名唯一性 |
| Service | `internal/service/user.go` | `Register(username, password string) error` | bcrypt 加密密码、唯一性校验 |
| Handler | `internal/handler/user.go` | `Register(c *gin.Context)` | 绑定请求 → 调用 service → 返回响应 |
| Router | `router/router.go` | 改为 `handler.Register` | 切换路由 |
| **清理** | `api/user.go` | **删除 `Register` 函数** | — |

##### 2.2 获取用户信息 `/api/user/info`

| 层 | 方法 | 说明 |
|----|------|------|
| Repository | `GetUserByID(id uint) (*model.User, error)` | 新增 |
| Service | `GetUserInfo(userID uint) (*model.User, error)` | 获取用户信息，清空密码 |
| Handler | `GetUserInfo(c *gin.Context)` | 从 JWT 取 userId |
| **清理** | 删除 `api/user.go` 中 `GetUserInfo` | — |

##### 2.3 退出登录 `/api/user/logout`

| 层 | 方法 | 说明 |
|----|------|------|
| Repository | `AddToBlacklist(token string, ttl time.Duration) error` | 新增，Redis 黑名单 |
| | `CheckBlacklist(token string) (bool, error)` | 新增，中间件用 |
| Service | `Logout(token string) error` | 调用 AddToBlacklist |
| Handler | `Logout(c *gin.Context)` | 从 Header 取 token |
| **清理** | 删除 `api/user.go` 中 `Logout` | — |

##### 2.4 修改密码 `/api/user/password`

| 层 | 方法 | 说明 |
|----|------|------|
| Repository | `UpdatePassword(userID uint, hashedPassword string) error` | 新增 |
| Service | `UpdatePassword(userID uint, oldPwd, newPwd string) error` | 校验旧密码 → bcrypt 新密码 |
| Handler | `UpdatePassword(c *gin.Context)` | — |
| **清理** | 删除 `api/user.go` 中 `UpdatePassword` | — |

##### 2.5 用户分页列表 `/api/user`

| 层 | 方法 | 说明 |
|----|------|------|
| Repository | `ListUsers(page, pageSize int) ([]model.User, int64, error)` | 新增，修复硬编码分页 |
| Service | `ListUsers(page, pageSize int) ([]model.User, int64, error)` | 支持前端传入 page/pageSize |
| Handler | `UserList(c *gin.Context)` | ShouldBindQuery 读取分页参数 |
| **清理** | 删除 `api/user.go` 中 `UserList` | — |

##### 2.6 编辑用户、删除用户、状态切换

| 功能 | Repository (新增) | Service (新增) | Handler | 清理 |
|------|------------------|----------------|---------|------|
| 编辑用户 | `UpdateUser(user *model.User) error` | `EditUser(id int, req) error` | `EditUser(c)` | 删 api |
| 删除用户 | `DeleteUser(id uint) error` | `DeleteUser(id uint) error` | `DeleteUser(c)` | 删 api |
| 状态切换 | `UpdateStatus(id uint, status int) error` | `SwitchStatus(id uint, status int) error` | `SwitchStatus(c)` | 删 api |

##### Step 3：用户模块全部迁移完成校验
- [ ] 所有用户路由全部绑定 `internal/handler`
- [ ] `api/user.go` 内部代码全部清空，确认无任何被调用函数
- [ ] Git 提交：`refactor: 完成用户模块迁移至 internal 三层架构`

---

### 第二阶段：迁移 AI 模块

#### 当前问题
`api/ai.go` 扁平实现，直接调用 `utils.CallAI`，无分层，无错误处理，无上下文。

#### Step 1：分层拆分 AI 逻辑

| 层 | 文件 | 方法 | 说明 |
|----|------|------|------|
| Repository | `internal/repository/ai_repo.go` | `CallLLM(prompt string) (string, error)` | 封装 LLM HTTP 请求、重试、超时；将 `utils/ai.go` 调用逻辑迁移至此；移除硬编码 Key，从 env 读取 |
| Service | `internal/service/ai_service.go` | `GenerateCode(prompt string) (string, error)` | Prompt 组装 |
| | | `ExplainCode(code string) (string, error)` | 结果后处理 |
| | | `FixCode(code string) (string, error)` | — |
| | | `OptimizeCode(code string) (string, error)` | — |
| Handler | `internal/handler/ai_handler.go` | `GenerateCode(c *gin.Context)` | 参数校验 |
| | | `ExplainCode(c *gin.Context)` | 调用 service |
| | | `FixCode(c *gin.Context)` | 返回结果 |
| | | `OptimizeCode(c *gin.Context)` | — |

#### Step 2：路由切换
- [ ] 4 条 AI 接口从 `api.GenerateCode` 切换至 `aiHandler.GenerateCode`
- [ ] 测试全部 AI 接口可用

#### Step 3：清理
- [ ] 删除 `api/ai.go`
- [ ] 至此整个 `api/` 目录无任何业务代码

---

### 第三阶段：收尾清理 & 架构固化

| # | 任务 | 说明 |
|---|------|------|
| 1 | **彻底删除顶层 `api/` 整个文件夹** | 里程碑节点 |
| 2 | 统一 `config.Ctx` | 移除各处零散的 `context.Background()`，统一使用 config 层的单例 |
| 3 | 清理重复 CORS 定义 | 统一使用 `middleware/cors.go` |
| 4 | 删除无效死代码 | `middleware/limiter.go` 等不再使用的中间件 |
| 5 | 统一 DTO 结构体 | 登录请求、分页请求、AI 请求结构体统一放入 `internal/model/`，分散在 handler 内的结构体全部收拢 |
| 6 | 编译全量测试 | `go build ./...` 全部通过 |
| 7 | 接口功能回归 | 所有接口测试正常 |
| 8 | Git 提交 | `refactor: 删除废弃 api 目录，完成全项目三层架构迁移` |

---

## 回归测试清单

每迁移完一个接口，执行以下验证：

```bash
# 编译检查
go build ./...

# 启动服务
go run cmd/main.go

# 测试接口
curl -X POST http://localhost:8080/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'

# 验证 JWT 鉴权
curl http://localhost:8080/api/user/info \
  -H "Authorization: Bearer <token>"
```

## 注意事项
1. 迁移完成一个接口，**立刻删除** `api/` 中对应的旧实现，不要保留两份代码
2. 不要跨多个接口同时修改，每次只聚焦一个接口的完整链路
3. 接口行为要完全一致：输入不变、输出不变、错误码不变
4. 每次路由切换后，用 curl 测试旧接口是否 404（确认旧路由已去除）