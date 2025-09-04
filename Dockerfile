# 多阶段构建 Dockerfile for chat-matcher
# 第一阶段：构建阶段
FROM golang:1.25.0-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o chat-matcher-server .

# 第二阶段：运行阶段
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates wget

# 创建非 root 用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件和静态文件
COPY --from=builder /app/chat-matcher-server ./chat-matcher-server
COPY --from=builder /app/static ./static

# 设置可执行权限
RUN chmod +x ./chat-matcher-server

# 修改文件所有者
RUN chown -R appuser:appgroup /app

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 9093

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9093/static/index.html || exit 1

# 启动应用
CMD ["./chat-matcher-server"]
