# 多阶段构建 Dockerfile
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 获取构建信息
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-X main.Version=${VERSION} \
    -X main.BuildTime=${BUILD_TIME} \
    -X main.GitCommit=${GIT_COMMIT} \
    -s -w" \
    -o anytls-server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-X main.Version=${VERSION} \
    -X main.BuildTime=${BUILD_TIME} \
    -X main.GitCommit=${GIT_COMMIT} \
    -s -w" \
    -o anytls-client ./cmd/client

# 最终镜像
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1001 anytls && \
    adduser -D -s /bin/sh -u 1001 -G anytls anytls

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/anytls-server .
COPY --from=builder /app/anytls-client .

# 复制配置文件和文档
COPY --from=builder /app/README.md .
COPY --from=builder /app/USAGE.md .
COPY --from=builder /app/examples ./examples

# 创建必要的目录
RUN mkdir -p /app/config /app/logs

# 更改所有权
RUN chown -R anytls:anytls /app

# 切换到非 root 用户
USER anytls

# 暴露端口
EXPOSE 8443 1080

# 设置环境变量
ENV TZ=Asia/Shanghai

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ./anytls-server -version || exit 1

# 启动命令
CMD ["./anytls-server", "-l", "0.0.0.0:8443", "-p", "yourpassword"]
