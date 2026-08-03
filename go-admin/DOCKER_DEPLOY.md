# go-admin Docker 部署工程化

本文档说明 `go-admin` 如何通过 Docker Compose 一键启动（应用 + MySQL + Redis），以及镜像构建与配置注入方式。

## 1. 架构总览

```
┌────────────────────────────── 宿主机 ──────────────────────────────┐
│                                                                   │
│   curl http://localhost:8080                                       │
│        │                                                          │
│        ▼                                                          │
│   ┌─────────────────── docker-compose 自定义网络 go-network ───┐  │
│   │                                                           │  │
│   │   ┌──────────────────────┐                                │  │
│   │   │ go-admin (8080 对外)  │── healthcheck: nc -w 3 :8080 < /dev/null     │  │
│   │   └──────────┬───────────┘                                │  │
│   │              │ depends_on: condition: service_healthy     │  │
│   │   ┌──────────▼───────────┐   ┌─────────────────────────┐  │  │
│   │   │ mysql:8.0            │   │ redis:alpine            │  │  │
│   │   │ healthcheck:         │   │ healthcheck: redis-cli   │  │  │
│   │   │ mysqladmin ping      │   │ ping                     │  │  │
│   │   │ volume: mysql-data   │   │ volume: redis-data       │  │  │
│   │   └──────────────────────┘   └─────────────────────────┘  │  │
│   └───────────────────────────────────────────────────────────┘  │
│                                                                   │
│   mysql / redis 不映射宿主端口（仅内网可达），避免与宿主 3306/6379 冲突 │
└───────────────────────────────────────────────────────────────────┘
```

- `go-admin`：应用容器，仅暴露 `8080` 端口到宿主机
- `mysql`：数据库，`mysql-data` 卷持久化数据
- `redis`：缓存，`redis-data` 卷持久化 AOF 数据，支持可选 `requirepass`
- 三个服务都配置了 `healthcheck`，`go-admin` 通过 `depends_on: condition: service_healthy` 严格等待 MySQL / Redis 就绪后才启动

## 2. Dockerfile 说明（多阶段构建）

| 阶段 | 基础镜像 | 职责 |
|------|----------|------|
| `builder` | `golang:1.25-alpine` | 下载依赖 + 静态编译（`CGO_ENABLED=0`，`-ldflags="-s -w"`） |
| `runtime` | `alpine:3.20` | 最小运行镜像，仅拷贝编译产物，非 root 用户运行 |

关键点：

1. **多阶段构建**：最终镜像不含 Go 工具链与源码，体积小、更安全
2. **非开发镜像运行**：运行阶段为 alpine 最小镜像，不含编译器
3. **镜像不包含 `.env`**：
   - `.dockerignore` 明确排除 `.env`（构建上下文不携带）
   - 最终镜像只 `COPY --from=builder /build/go-admin` 单个二进制
4. **支持环境变量注入**：运行镜像不写死任何配置，`ENTRYPOINT ["./go-admin"]` 直接读取进程环境变量
5. 运行镜像安装 `ca-certificates`（AI 外部 HTTPS 调用）与 `tzdata`（DSN `loc=Local`）

## 3. 配置注入矩阵（全部来自环境变量）

| 分类 | 环境变量 | 来源（docker-compose） |
|------|----------|------------------------|
| MySQL | `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | `${DB_HOST:-mysql}` 等，默认指向 compose 网络内 `mysql` |
| Redis | `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD` / `REDIS_DB` / `REDIS_PREFIX` / `REDIS_POOL_SIZE` / `REDIS_MIN_IDLE_CONNS` / `REDIS_TLS_ENABLE` | `${REDIS_HOST:-redis}` 等 |
| AI | `AI_API_KEY` / `AI_ENDPOINT` / `AI_URL` / `AI_TIMEOUT` | `${AI_API_KEY:-}`，生产通过 `.env` 或宿主环境注入真实 Key |
| JWT | `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET` | 生产务必覆盖默认值 |
| 运行 | `ENV`（dev/prod）/ `GIN_MODE` / `TZ` | `${ENV:-dev}` 等 |

## 4. 启动步骤

```bash
cd go-admin

# 1) 构建镜像
docker compose build

# 2) 启动全部服务（-d 后台；不带 -d 可查看实时启动日志）
docker compose up -d

# 3) 查看状态与日志
docker compose ps
docker compose logs -f go-admin

# 4) 停止
docker compose down
```

> 注意：`docker compose` 需要 Docker Desktop / Docker Engine（Compose v2）。

## 5. 验证

```powershell
# Windows PowerShell：一键构建 + 启动 + 健康检查 + register/login 验证
.\verify-docker.ps1

# 或手动 curl（Linux/macOS）
curl -s -X POST http://localhost:8080/api/user/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"docker_test","password":"123456"}'

curl -s -X POST http://localhost:8080/api/user/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"docker_test","password":"123456"}'
```

## 6. 生产环境建议

- 通过宿主环境或 `.env`（compose 自动读取）覆盖所有 `${VAR:-default}`：
  - 设置 `ENV=prod`（应用强制 Redis 密码非空）
  - 设置 `REDIS_PASSWORD=<强密码>`、`DB_PASSWORD=<强密码>`
  - 设置 `AI_API_KEY=<真实 Key>`、`JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET` 强随机值
- 如需要从宿主机直连 MySQL / Redis，可自行在对应服务增加 `ports: - "3306:3306"`（注意宿主端口冲突）
