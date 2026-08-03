# go-admin CI/CD 说明

GitHub Actions 工作流：`.github/workflows/ci.yml`

## 1. 触发条件

- `push`：任意分支推送
- `pull_request`：任意 PR

同一分支的新提交会取消旧的在跑 job（`concurrency`）。

## 2. 流水线（3 个 Job）

### Job 1 `go-ci` —— Go 质量门禁

| 步骤 | 命令 | 说明 |
|------|------|------|
| 1 | `go version` | 打印 Go 版本（由 `go.mod` 自动决定，`go-version-file`） |
| 2 | `go mod download` | 下载依赖 |
| 3 | `go test ./...` | 全部测试。**内置 MySQL + Redis 服务容器**，接口自动化测试真实执行（非跳过） |
| 4 | `go vet ./...` | 静态检查 |
| 5 | `go build ./...` | 全量编译验证 |

测试隔离：
- 数据库：使用独立 schema `go_admin_test`（由 `TEST_DB_NAME` 决定），CI 内置 MySQL 自动创建
- Redis：使用独立 key 前缀 `test`（`TEST_REDIS_PREFIX`），与生产 key 完全隔离
- 测试环境变量（`DB_HOST=127.0.0.1` 等）指向服务容器，覆盖 CI 中不存在的 `.env`

### Job 2 `docker-build` —— Docker 镜像构建

- 使用仓库已有多阶段 `Dockerfile`（`./go-admin/Dockerfile`），不修改业务代码
- 仅构建不推送（`push: false`），Tag `go-admin:ci`
- 启用 Buildx + GHA 层缓存，二次构建复用缓存

### Job 3 `security` —— 安全门禁

自包含 bash 扫描（无第三方 Action 依赖），失败则整个 CI 失败：

1. `.env`（及 `.env.*`）不得被 git 跟踪 / 不得出现在检出工作区
2. 不得出现私钥块（`BEGIN ... PRIVATE KEY`）、AWS 访问密钥（`AKIA...`）、GitHub PAT（`ghp_...`）、Stripe 密钥（`sk_live_...`）
3. 敏感配置项（`AI_API_KEY` / `AI_ENDPOINT` / `DB_PASSWORD` / `REDIS_PASSWORD` / `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET`）只允许 `${VAR:-default}` 占位或空值，禁止 16 位以上真实值

## 3. 本地等价验证

```bash
cd go-admin
go version
go mod download
go test ./...      # 本机无 MySQL/Redis 时自动跳过集成测试
go vet ./...
go build ./...
docker build -t go-admin:ci .   # 需要 Docker
```

## 4. 安全约定

- `.env` 已在 `.gitignore` 中排除，**严禁 `git add -f .env`**
- API Key / 数据库密码 / JWT 密钥一律通过环境变量或 GitHub Secrets 注入
- 提交前可用 Job 3 的扫描规则自查
