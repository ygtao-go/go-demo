# go-admin

一个基于 **Go + Gin + GORM + Redis + JWT** 的后台管理系统工程，内置 **AI 代码助手模块**。项目以生产工程为标准完成建设：三层架构、Redis 缓存治理（穿透/击穿/雪崩防护）、JWT 双 Token 安全、Prometheus 监控、自动化测试、Docker 部署与 GitHub Actions CI/CD。

> 仓库结构：`go-admin/` 为后端工程，`web/index.html` 为演示前端（Vue3 + Element Plus 单页）。

## 目录

- [项目介绍](#项目介绍)
- [技术栈](#技术栈)
- [架构说明](#架构说明)
- [项目目录](#项目目录)
- [核心技术设计](#核心技术设计)
- [快速启动](#快速启动)
- [测试方式](#测试方式)
- [Docker 部署](#docker-部署)
- [CI/CD 说明](#cicd-说明)
- [监控说明](#监控说明)
- [Swagger 接口文档](#swagger-接口文档)
- [文档导航](#文档导航)

---

## 项目介绍

| 能力 | 说明 |
|------|------|
| 用户模块 | 注册 / 登录 / 用户信息 / 退出 / 修改密码 / 用户管理（列表、编辑、删除、状态切换） |
| 安全认证 | JWT 双 Token（Access 15min + Refresh 7 天），Refresh Token 旋转 + 黑名单 + 原子消费 |
| Redis 缓存治理 | 用户缓存 + 不存在缓存（防穿透）、singleflight 单飞（防击穿）、TTL 随机化（防雪崩） |
| 分布式限流 | 基于 Redis 的 INCR + 过期时间窗口限流（用户/IP × 接口粒度，默认 60 次/分钟） |
| AI 模块 | 豆包（火山方舟 Chat Completions）代码生成 / 解释 / 修复 / 优化，配置全部环境变量化 |
| 可观测性 | RequestID 全链路日志 + Prometheus HTTP/业务指标 |
| 工程质量 | 自动化测试、多阶段 Docker 镜像、Docker Compose 一键部署、GitHub Actions CI/CD、安全门禁 |

## 技术栈

| 分类 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.25 |
| Web 框架 | [Gin](https://github.com/gin-gonic/gin) | v1.12 |
| ORM | [GORM](https://gorm.io) + mysql 驱动 | v1.31 |
| 缓存 | [go-redis](https://github.com/go-redis/redis) | v8.11.5 |
| JWT | [golang-jwt/jwt](https://github.com/golang-jwt/jwt) | v4.5.2 |
| 密码加密 | golang.org/x/crypto/bcrypt | — |
| 并发原语 | golang.org/x/sync（singleflight） | v0.22 |
| AI 服务 | 火山方舟（豆包）Chat Completions API | — |
| 监控 | prometheus/client_golang | v1.24.1 |
| 配置加载 | joho/godotenv | v1.5.1 |
| 数据库 | MySQL | 8.0 |
| 部署 | Docker（多阶段构建）+ Docker Compose | — |
| CI/CD | GitHub Actions | — |
| 测试 | Go testing + httptest + prometheus/testutil | — |

## 架构说明

项目采用经典 **三层架构**，并叠加一层全局中间件链：

```
HTTP 请求
   │
   ▼
[中间件链] RequestID → Metrics → Logger → RedisLimit → Recovery → CORS
   │
   ▼
router.InitRouter  ──►  /metrics（Prometheus 采集端点，无需 JWT）
                        /api（公开组：register / login / refresh）
                        /api + JWTAuth（私有组：user/*、ai/*）

## 项目目录

```
go-admin/
├── cmd/
│   └── main.go                  # 应用入口：加载 .env、初始化 MySQL/Redis、挂载中间件与路由
├── config/
│   ├── db.go                    # MySQL 连接（GORM）+ 自动建表
│   ├── redis.go                 # Redis 客户端 / 连接池 / 环境前缀 / 超时上下文
│   └── ai.go                    # AI（豆包/火山方舟）配置（懒加载 + 幂等）
├── internal/
│   ├── dto/                     # 请求 DTO（user.go / ai.go）
│   ├── handler/                 # Handler 层：HTTP 输入输出
│   │   ├── user.go
│   │   └── ai_handler.go
│   ├── service/                 # Service 层：业务逻辑
│   │   ├── user.go
│   │   └── ai_service.go
│   └── repository/              # Repository 层：数据访问
│       ├── user.go              #   用户 CRUD + Redis 缓存 + JTI 黑名单 + Refresh Token 管理
│       ├── user_cache_test.go   #   TTL 随机化单元测试
│       └── ai_repository.go     #   AI Provider 唯一入口（HTTP 调用）
├── middleware/                  # 中间件
│   ├── auth.go                  # JWT 鉴权（校验签名/过期/类型/黑名单）
│   ├── cors.go                  # CORS（全项目唯一实现）
│   ├── logger.go                # RequestID 生成 + 请求日志
│   ├── recovery.go              # panic 恢复（按环境返回详细/脱敏错误）
│   └── redis_limit.go           # Redis 分布式限流（用户/IP × 接口）
├── model/
│   └── user.go                  # User 实体
├── pkg/
│   ├── metrics/                 # Prometheus 指标（metrics.go / http.go / metrics_test.go）
│   └── response/                # 统一响应信封 {code, msg, data}
├── router/
│   └── router.go                # 路由注册（公开组 / 私有组 / /metrics）
├── utils/
│   ├── jwt.go                   # JWT 双 Token 生成 / 解析 / JTI
│   └── convert.go               # 兼容工具函数
├── tests/                       # 接口自动化测试（隔离环境）
│   ├── main_test.go
│   └── auth_test.go
├── docs/                        # 项目文档（本目录）
├── .env                         # 本地环境变量（已被 .gitignore 排除，严禁入库）
├── .dockerignore                # 构建上下文排除清单
├── Dockerfile                   # 多阶段构建
├── docker-compose.yml           # 应用 + MySQL + Redis 一键编排
├── test.http                    # HTTP 接口调试脚本
└── verify-docker.ps1            # Windows 一键 Docker 验证脚本
```

## 核心技术设计

| 设计 | 一句话说明 | 文档 |
|------|-----------|------|
| JWT 双 Token | Access（15min）+ Refresh（7 天）独立密钥；Refresh 旋转 + Redis JTI 黑名单 + Lua 原子消费 | [docs/DESIGN.md](docs/DESIGN.md#2-refresh-token-设计) |
| Redis 缓存治理 | none 缓存防穿透、singleflight 防击穿、TTL ±1h 随机化防雪崩；缓存不存密码，写路径主动失效 | [docs/DESIGN.md](docs/DESIGN.md#1-redis-缓存设计) |
| AI 模块 | 三层封装，repository 为 AI Provider 唯一入口；配置全环境变量化；双层超时；失败即上报指标 | [docs/DESIGN.md](docs/DESIGN.md#3-ai-模块设计) |
| 分布式限流 | Redis INCR + 首次设置过期时间，按 用户/IP × 接口 维度，默认 60 次/分钟 | [docs/DESIGN.md](docs/DESIGN.md#15-分布式限流) |
| 可观测性 | RequestID 全链路 + Prometheus HTTP/业务指标 | [docs/MONITORING.md](docs/MONITORING.md) |

## 快速启动


### 3. 启动服务

```bash
cd go-admin
go mod download
go run ./cmd/main.go
```

服务默认监听 `:8080`。启动成功日志：

```
✅ MySQL 连接成功，用户表已就绪！
Redis连接成功 ✅ addr=127.0.0.1:6379 db=0
服务启动成功，监听端口:8080
```

### 4. 快速验证

```bash
# 注册
curl -s -X POST http://localhost:8080/api/user/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"123456"}'

# 登录（返回 accessToken / refreshToken）
curl -s -X POST http://localhost:8080/api/user/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"123456"}'
```

也可以直接用 `test.http`（VS Code REST Client 插件）或 `web/index.html`（浏览器打开，后端已全局开启 CORS）联调。

## 测试方式

```bash
cd go-admin
go test ./...
```

| 测试 | 位置 | 依赖 | 说明 |
|------|------|------|------|
| 缓存 TTL 随机化 | `internal/repository/user_cache_test.go` | 无 | 纯单元测试 |
| 缓存三防（穿透/击穿/雪崩） | `internal/service/user_cache_test.go` | MySQL + Redis | 不可用时自动跳过（`t.Skip`） |
| Prometheus 指标 | `pkg/metrics/metrics_test.go` | 无 | 中间件与业务指标上报 |
| 接口自动化 | `tests/`（`auth_test.go`） | MySQL + Redis | 注册/登录/信息/刷新/登出、Refresh 旋转、并发刷新原子性、token 类型隔离 |

**测试隔离方案**（见 `tests/main_test.go`）：

- 数据库：独立 schema `go_admin_test`（`TEST_DB_NAME` 可覆盖），运行前后清空 `users` 表，不触碰生产库；
- Redis：独立 key 前缀 `test`（`TEST_REDIS_PREFIX` 可覆盖），运行前后扫描并删除 `test:*`；
- 基础设施不可达时自动跳过（不 fail），保证无 MySQL/Redis 的环境仍可编译运行。

## Docker 部署

使用多阶段 Dockerfile + Docker Compose 一键启动（应用 + MySQL + Redis）：

```bash
cd go-admin

# 构建并启动全部服务（后台运行）
docker compose up -d

# 查看状态与日志
docker compose ps
docker compose logs -f go-admin

# 停止
docker compose down
```

- 应用服务暴露 `8080`；MySQL、Redis 仅内网可达（不映射宿主端口），数据分别持久化到 `mysql-data`、`redis-data` 卷；
- 三个服务均配置 `healthcheck`，应用通过 `depends_on: condition: service_healthy` 等待依赖就绪；
- 所有配置通过环境变量注入（`${VAR:-default}` 占位），`.env` 被 `.dockerignore` 排除，不会进入镜像；
- 生产环境建议：`ENV=prod`、设置 `REDIS_PASSWORD` / `DB_PASSWORD` 强密码、注入真实 `AI_API_KEY` 并覆盖 JWT 密钥。

Windows 一键验证脚本：

```powershell
cd go-admin
.\verify-docker.ps1
```

详细说明见 `DOCKER_DEPLOY.md` 与 `docker-compose.yml` 注释。

## CI/CD 说明

GitHub Actions 工作流：`.github/workflows/ci.yml`（仓库根目录），`push` / `pull_request` 触发，共 3 个 Job：

| Job | 内容 |
|-----|------|
| `go-ci` | Go 质量门禁：`go version` → `go mod download` → `go test ./...` → `go vet ./...` → `go build ./...`。内置 MySQL + Redis 服务容器，让接口自动化测试真实执行（而非跳过） |
| `docker-build` | 使用多阶段 Dockerfile 构建 `go-admin:ci` 镜像（仅构建不推送），启用 Buildx + GHA 层缓存 |
| `security` | 安全门禁：`.env` 不得入库 / 无私钥与密钥指纹 / 敏感配置项只允许占位符，禁止 16 位以上真实值 |

本地等价验证：

```bash
cd go-admin
go version && go mod download && go test ./... && go vet ./... && go build ./...
```

详细说明见 `CI.md`。

## 监控说明

Prometheus 指标通过 **HTTP 中间件自动采集 + 业务代码手动上报** 两种方式产生，统一暴露在 `GET /metrics`（无需 JWT）：

| 指标 | 类型 | 维度 | 说明 |
|------|------|------|------|
| `http_requests_total` | Counter | method / path / status | HTTP 请求总数 |
| `http_request_duration_seconds` | Histogram | method / path | 请求耗时（默认桶） |
| `http_errors_total` | Counter | method / path / status | HTTP 错误数（status ≥ 400） |
| `refresh_success_total` | Counter | — | Refresh Token 刷新成功次数 |
| `refresh_failure_total` | Counter | — | Refresh Token 刷新失败次数 |
| `ai_calls_total` | Counter | — | AI 服务调用总次数 |
| `ai_failures_total` | Counter | — | AI 服务调用失败次数 |

采集配置示例（Prometheus）：

```yaml
scrape_configs:
  - job_name: go-admin
    metrics_path: /metrics
    static_configs:
      - targets: ["localhost:8080"]
```

详细说明（指标语义、上报位置、PromQL 示例）见 [docs/MONITORING.md](docs/MONITORING.md)。

## Swagger 接口文档

项目已集成 [swaggo/gin-swagger](https://github.com/swaggo/gin-swagger) + [swaggo/swag](https://github.com/swaggo/swag)，自动生成 OpenAPI（Swagger 2.0）接口文档，覆盖全部 14 个业务接口。

### 访问地址

服务启动后访问（`/swagger/*` 为公开端点，与 `/metrics` 一样无需 JWT）：

| 地址 | 说明 |
|------|------|
| http://localhost:8080/swagger/index.html | Swagger UI（可视化调试） |
| http://localhost:8080/swagger/doc.json | OpenAPI JSON（Swagger UI 加载的规范） |

> 说明：gin-swagger v1.6.0 通过 HTTP 仅暴露 JSON 规范（`doc.json`）；OpenAPI YAML 原始文件见仓库 `docs/swagger.yaml`，可用于 CI 静态校验或第三方工具导入。

### 使用方式

1. **公开接口**（注册 / 登录 / 刷新 Token）：直接在 Swagger UI 中点击对应接口 → `Try it out` → `Execute` 调用；
2. **私有接口**（需 JWT）：先调用 `POST /api/user/login` 获取 `data.accessToken`，然后点击页面右上角 `Authorize`，输入 `Bearer <accessToken>` 后即可调试全部需鉴权接口（用户管理 / AI 模块）；
3. `accessToken` 有效期 15 分钟，过期后重新登录或调用 `POST /api/user/refresh` 换新。

### 重新生成文档

修改接口注释（`internal/handler/*.go`）或 API 元信息（`cmd/main.go`）后，在 `go-admin/` 目录执行：

```bash
swag init -g cmd/main.go -o docs --parseDependency --parseInternal
```

生成文件 `docs/docs.go`、`docs/swagger.json`、`docs/swagger.yaml` 提交到仓库。

### 接口清单

| 模块 | 路径 | 方法 | 鉴权 |
|------|------|------|------|
| 用户 | /api/user/register | POST | 否 |
| 用户 | /api/user/login | POST | 否 |
| 用户 | /api/user/refresh | POST | 否 |
| 用户 | /api/user/info | GET | Bearer |
| 用户 | /api/user/logout | POST | Bearer |
| 用户 | /api/user/password | PUT | Bearer |
| 用户 | /api/user | GET | Bearer |
| 用户 | /api/user/{id} | PUT | Bearer |
| 用户 | /api/user/{id} | DELETE | Bearer |
| 用户 | /api/user/{id}/status | PATCH | Bearer |
| AI | /api/ai/generate | POST | Bearer |
| AI | /api/ai/explain | POST | Bearer |
| AI | /api/ai/fix | POST | Bearer |
| AI | /api/ai/optimize | POST | Bearer |

## 文档导航

| 文档 | 内容 |
|------|------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 三层架构说明与请求流程 |
| [docs/DESIGN.md](docs/DESIGN.md) | Redis 缓存 / Refresh Token / AI 模块设计 |
| [docs/API.md](docs/API.md) | 接口列表与字段说明 |
| [docs/MONITORING.md](docs/MONITORING.md) | Prometheus 指标说明 |
| `docs/docs.go` / `docs/swagger.json` / `docs/swagger.yaml` | Swagger/OpenAPI 自动生成文件（`swag init` 产物） |
| `CI.md` | CI/CD 流水线说明 |
| `DOCKER_DEPLOY.md` | Docker 部署工程化说明 |
| `JWT_ANALYSIS.md` / `REDIS_ANALYSIS.md` / `MIGRATION_PLAN.md` | 历史审计 / 迁移分析报告 |

### 1. 环境要求

- Go 1.25+
- MySQL 8.0（或通过 Docker Compose 启动）
- Redis 6+（或通过 Docker Compose 启动）

### 2. 配置环境变量

本地开发在 `go-admin/.env` 中配置（参考 `config/*.go` 中的环境变量说明；`.env` 已被 git 排除，不会入库）：

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `ENV` | 否 | `dev` | `dev` 允许 Redis 无密码；`prod` 强制 Redis 密码非空（否则启动失败） |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | 是 | — | MySQL 连接信息 |
| `REDIS_HOST` / `REDIS_PORT` | 是 | `127.0.0.1` / `6379` | Redis 地址 |
| `REDIS_PASSWORD` | 否 | 空 | 生产必须设置强密码 |
| `REDIS_PREFIX` | 否 | 空 | Redis key 环境前缀（如 `dev`/`prod`），实现多环境 key 隔离 |
| `REDIS_DB` / `REDIS_POOL_SIZE` / `REDIS_MIN_IDLE_CONNS` | 否 | `0` / `20` / `5` | Redis 逻辑库与连接池 |
| `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET` | 否 | 内置默认值 | 生产务必覆盖为强随机值 |
| `AI_API_KEY` / `AI_ENDPOINT` | 是（使用 AI 时） | — | 豆包 API Key 与接入点 ID，严禁硬编码 |
| `AI_URL` / `AI_TIMEOUT` | 否 | 火山方舟地址 / 30 | AI 接口地址与超时（秒） |

   │
   ▼
Handler 层  internal/handler    ：解析请求 DTO、调用 Service、输出统一响应
   │
   ▼
Service 层  internal/service    ：业务规则、密码校验、缓存三防、Token 生成/刷新
   │
   ▼
Repository 层 internal/repository：MySQL（GORM）与 Redis 读写、第三方 AI HTTP 调用
   │
   ├──► config.DB（GORM / MySQL）
   ├──► config.RedisClient（go-redis）
   └──► 火山方舟 Chat Completions API
```

- **Handler 层**只做 HTTP 输入输出，不写业务逻辑；
- **Service 层**只做业务规则，不直接访问数据源；
- **Repository 层**只做数据访问（DB / Redis / 第三方 AI），并承担 AI Provider 调用的唯一入口；
- **DTO 层**（`internal/dto`）统一定义请求结构体，**Model 层**（`model`）定义数据实体；
- 所有中间件与配置（MySQL / Redis / AI / JWT）均来自环境变量，无硬编码敏感信息。

详细说明（分层职责、依赖规则、完整请求时序图）见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。
