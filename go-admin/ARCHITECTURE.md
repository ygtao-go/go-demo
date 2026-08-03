# go-admin 架构分析：router → handler → service → repository 关系

## 一、整体调用链

```
cmd/main.go  (应用入口)
  │
  ├─ config.InitDB()     (初始化 MySQL)
  ├─ config.InitRedis()  (初始化 Redis)
  │
  └─ router.InitRouter(r)  (注册路由)
        │
        └─ gin 收到请求 → 路由匹配 → 调用 handler 函数
```

## 二、路由层 (router/router.go)

**文件**: `go-admin/router/router.go`

路由层定义了 URL 路径到 handler 函数的映射关系，分两组：

| 分组 | 中间件 | 路由 | Handler |
|------|--------|------|---------|
| `public` (公开) | Cors | `POST /api/user/register` | `api.Register` |
| | | `POST /api/user/login` | `api.Login` |
| `auth` (需 JWT) | Cors → JWTAuth | `GET /api/user/info` | `api.GetUserInfo` |
| | | `POST /api/user/logout` | `api.Logout` |
| | | `PUT /api/user/password` | `api.UpdatePassword` |
| | | `GET /api/user` | `api.UserList` |
| | | `PUT /api/user/:id` | `api.EditUser` |
| | | `DELETE /api/user/:id` | `api.DeleteUser` |
| | | `PATCH /api/user/:id/status` | `api.SwitchStatus` |
| | | `POST /api/ai/generate` | `api.GenerateCode` |
| | | `POST /api/ai/explain` | `api.ExplainCode` |
| | | `POST /api/ai/fix` | `api.FixCode` |
| | | `POST /api/ai/optimize` | `api.OptimizeCode` |

**关键点**: 所有路由都指向 `api/` 包（`api.Register`、`api.Login` 等），**没有任何路由指向 `internal/handler/` 包**。

## 三、两套并存的 Handler 架构

当前项目中存在 **两套不同的代码架构**：

### 架构 A：扁平架构（实际使用）—— `api/` 包

```
router/router.go
    │
    ▼
api/user.go ───→ config.DB (直接操作 MySQL)
              └──→ config.RedisClient (直接操作 Redis)

api/ai.go ───→ utils.CallAI (直接调用 AI)
```

**特点**:
- Handler 就是"胖函数"
- 直接在 Handler 里写 SQL 查询、密码加密、Redis 缓存
- 没有 service 层、没有 repository 层
- 代码耦合度高，难以单独测试

**示例（api/user.go:Login）**:
```go
func Login(c *gin.Context) {
    // 1. 解析请求
    var req LoginReq
    c.ShouldBindJSON(&req)

    // 2. 直接查询 Redis
    config.RedisClient.Get(c, cacheKey).Result()

    // 3. 直接查询 DB
    config.DB.Where("username = ?", req.Username).First(&user)

    // 4. 直接加密比较
    bcrypt.CompareHashAndPassword(...)

    // 5. 直接生成 Token（硬编码的简易实现）
    GenerateToken(user.ID)

    // 6. 直接写 Redis 缓存
    config.RedisClient.Set(c, cacheKey, data, 24*time.Hour)

    // 7. 直接返回响应
    response.Success(c, token)
}
```

### 架构 B：三层架构（已定义但未启用）—— `internal/` 包

```
router/router.go  ←── 未引用！路由不指向这里
    │
    ▼  (理论上)
internal/handler/user.go
    │  调用 service
    ▼
internal/service/user.go
    │  调用 repository
    ▼
internal/repository/user.go ───→ config.DB
                            └──→ config.RedisClient
```

**特点**:
- **Handler 层** (`internal/handler/`)：只负责 HTTP 输入输出（解析请求、返回响应）
- **Service 层** (`internal/service/`)：只负责业务逻辑（校验密码、生成 token、触发缓存）
- **Repository 层** (`internal/repository/`)：只负责数据访问（DB 查询、Redis 读写）
- 每一层职责单一，可独立测试和 Mock

**示例**:
```go
// === Handler 层 (internal/handler/user.go) ===
func Login(c *gin.Context) {
    var req LoginReq
    c.ShouldBindJSON(&req)
    token, err := service.Login(req.Username, req.Password)
    if err != nil {
        response.Fail(c, 400, err.Error())
        return
    }
    response.Success(c, token)
}

// === Service 层 (internal/service/user.go) ===
func Login(username, password string) (string, error) {
    user, err := repository.GetUserByUsername(username)
    // 密码验证...
    token := utils.GenerateToken(user.ID)
    go repository.CacheUser(user, 24*time.Hour)
    return token, nil
}

// === Repository 层 (internal/repository/user.go) ===
func GetUserByUsername(username string) (*model.User, error) {
    var user model.User
    err := config.DB.Where("username = ?", username).First(&user).Error
    return &user, err
}
```

## 四、依赖关系全景图

```
┌─────────────────────────────────────────────────────────────────┐
│                        cmd/main.go                              │
│  • 加载 .env                                                    │
│  • config.InitDB()  ──→ 创建 MySQL 连接，存入 config.DB         │
│  • config.InitRedis() ──→ 创建 Redis 连接，存入 config.RedisClient│
│  • 注册中间件 (Logger, RedisLimit, Recovery)                     │
│  • router.InitRouter(r)                                         │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                      router/router.go                           │
│  • 定义 14 条路由                                                │
│  • 所有路由指向 api 包 (api.Register, api.Login, ...)            │
└──────────────────────┬──────────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          ▼                         ▼
┌───────────────────┐   ┌──────────────────────┐
│  api/user.go      │   │  api/ai.go           │
│  (实际使用的      │   │  (实际使用的         │
│   胖 Handler)     │   │   胖 Handler)        │
│                   │   │                      │
│  直接操作:        │   │  直接调用:           │
│  ├─ config.DB     │   │  ├─ utils.CallAI     │
│  └─ config.       │   │                      │
│     RedisClient   │   │                      │
└────────┬──────────┘   └──────────────────────┘
         │
         │ (未使用，代码已写但路由不指向)
         ▼
┌──────────────────────────────────────────────────────────────────┐
│              internal/ 三层架构（闲置状态）                       │
│                                                                  │
│  ┌──────────────────────┐                                       │
│  │ internal/handler/    │                                       │
│  │  • user.go - Login   │                                       │
│  │    调用 service       │                                       │
│  └─────────┬────────────┘                                       │
│            │                                                     │
│            ▼                                                     │
│  ┌──────────────────────┐                                       │
│  │ internal/service/    │                                       │
│  │  • user.go - Login   │                                       │
│  │    调用 repository    │                                       │
│  └─────────┬────────────┘                                       │
│            │                                                     │
│            ▼                                                     │
│  ┌──────────────────────┐   ┌──────────────────┐                │
│  │ internal/repository/ │   │  model/          │                │
│  │  • user.go           │   │  • user.go       │                │
│  │    GetUserByUsername │   │  (数据实体)       │                │
│  │    CacheUser         │   └──────────────────┘                │
│  │    ├─ config.DB      │                                       │
│  │    └─ config.Redis.  │                                       │
│  │       Client         │                                       │
│  └──────────────────────┘                                       │
└──────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│  config/ 层（全局配置，所有层都依赖）                                │
│  • db.go  : var DB *gorm.DB (全局变量)                              │
│  • redis.go: var RedisClient *redis.Client, var Ctx context.Context│
│                                                                    │
│  被依赖关系：                                                        │
│  api/user.go     ──→ 直接引用 config.DB / config.RedisClient       │
│  repository/     ──→ 直接引用 config.DB / config.RedisClient       │
│  middleware/     ──→ 直接引用 config.RedisClient                    │
└─────────────────────────────────────────────────────────────────────┘
```

## 五、关键发现与结论

### 1. 两套架构共存但未统一
| 维度 | `api/`（扁平架构） | `internal/`（三层架构） |
|------|------------------|----------------------|
| **状态** | ✅ 路由实际连接 | ❌ 代码已写但路由未连接 |
| **职责分离** | ❌ handler 兼做所有事 | ✅ handler/service/repository 各司其职 |
| **可测试性** | ❌ 难以 Mock | ✅ 每层可独立测试 |
| **代码复用** | ❌ 逻辑写在 handler 内，其他 handler 无法复用 | ✅ service/repository 可被多个 handler 调用 |

### 2. 需要重构才能激活三层架构
要让 `internal/` 生效，需要做：
- 将 `router/router.go` 中的 `api.xxx` 替换为 `handler.xxx`
- 补全 `internal/handler/` 中缺失的 handler（目前只有 Login）
- 补全 `internal/service/` 中缺失的 service 方法
- 补全 `internal/repository/` 中缺失的 repository 方法

### 3. AI 模块未做分层
- `api/ai.go` 直接调用 `utils.CallAI`，无 service/repository 封装
- 如果按三层架构扩展，应该在 `internal/service/` 中增加 ai service，在 `internal/repository/` 中增加 ai repository（对 AI API 的调用封装）

### 4. 现有代码的问题
- `api/user.go` 中的 `UserList` 硬编码 page=1, pageSize=10，无分页参数
- `api/user.go` 中的 `GenerateToken` 是简易实现（`fmt.Sprintf("token-%d", userID)`），不是真正的 JWT
- `internal/handler/user.go` 仅实现了 Login，其他 handler 未实现
- 两套代码都操作 `config.DB` / `config.RedisClient` 全局变量，隐式耦合

### 5. 建议演进方向
```
短期（立即修复）：
  统一使用 internal/ 三层架构，废弃 api/ 包

中期（功能增强）：
  - internal/service/ 层增加事务管理
  - internal/repository/ 层增加接口（interface），支持 Mock 测试

长期（架构优化）：
  - 引入 DI（依赖注入），避免全局 config.DB/RedisClient 变量
  - 增加单元测试和集成测试覆盖率