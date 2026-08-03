# syntax=docker/dockerfile:1

# ==============================================================
# 阶段 1 —— 构建阶段（builder）
# 使用完整 Go 开发镜像编译静态二进制；本阶段产物不会进入最终镜像
# ==============================================================
FROM golang:1.25-alpine AS builder

# CGO_ENABLED=0：静态编译，运行阶段无需 libc
ENV GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

# 1) 先复制依赖清单 → 充分利用 Docker 层缓存（源码不变时不重复下载依赖）
COPY go.mod go.sum ./
RUN go mod download

# 2) 复制全部源码并编译（.dockerignore 已排除 .env / 测试 / 文档）
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o go-admin ./cmd/main.go

# ==============================================================
# 阶段 2 —— 运行阶段（runtime）
# 最小化 alpine 运行镜像：非开发镜像，无源码 / 无 Go 工具链 / 无 .env
# ==============================================================
FROM alpine:3.20

# ca-certificates：调用外部 HTTPS 服务（豆包 AI）
# tzdata：DSN 使用 loc=Local，时区数据由 TZ 环境变量决定
# 创建非 root 运行用户 app（安全加固）
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

WORKDIR /app

# 仅拷贝编译产物，不拷贝源码 / .env / 测试 / 文档
COPY --from=builder /build/go-admin .

USER app

EXPOSE 8080

# 所有配置（DB / Redis / AI / JWT）统一通过 docker-compose environment 注入
ENTRYPOINT ["./go-admin"]
