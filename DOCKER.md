# Chat-Matcher Docker 部署指南

本文档介绍如何使用 Docker 部署 chat-matcher 聊天匹配系统。

## 📁 Docker 相关文件

- `Dockerfile` - Docker 镜像构建文件
- `docker-compose.yml` - Docker Compose 配置文件
- `.dockerignore` - Docker 构建时忽略的文件
- `docker.sh` - 便捷的构建和运行脚本
- `DOCKER.md` - 本文档

## 🚀 快速开始

### 方法一：使用便捷脚本（推荐）

```bash
# 构建镜像
./docker.sh build

# 运行容器
./docker.sh run

# 查看日志
./docker.sh logs

# 停止容器
./docker.sh stop

# 重启容器
./docker.sh restart

# 清理资源
./docker.sh clean
```

### 方法二：使用 Docker Compose

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down

# 重新构建并启动
docker-compose up --build -d
```

### 方法三：手动 Docker 命令

```bash
# 构建镜像
docker build -t chat-matcher:latest .

# 运行容器
docker run -d \
  --name chat-matcher \
  -p 8080:8080 \
  --restart unless-stopped \
  chat-matcher:latest

# 查看容器状态
docker ps

# 查看日志
docker logs -f chat-matcher

# 停止容器
docker stop chat-matcher

# 删除容器
docker rm chat-matcher
```

## 🌐 访问应用

容器启动后，可以通过以下地址访问：

- **Web 界面**: http://localhost:8080/static/index.html
- **WebSocket 端点**: ws://localhost:8080/ws
- **匹配 API**: http://localhost:8080/match

## 🔧 Dockerfile 特性

### 多阶段构建
- **构建阶段**: 使用 `golang:1.24-alpine` 编译 Go 应用
- **运行阶段**: 使用轻量级 `alpine:latest` 运行应用
- **优势**: 最终镜像体积小，安全性高

### 安全特性
- 创建非 root 用户运行应用
- 只暴露必要的端口 (8080)
- 包含 CA 证书支持 HTTPS

### 健康检查
- 每 30 秒检查一次应用状态
- 通过访问静态页面验证服务可用性
- 支持容器自动重启

## 📊 资源使用

### 镜像大小
- 构建镜像: ~300MB（包含 Go 编译环境）
- 运行镜像: ~20MB（仅包含应用和 Alpine Linux）

### 运行资源
- 内存使用: ~10-50MB
- CPU 使用: 极低（事件驱动架构）
- 存储: ~20MB

## 🔍 故障排查

### 常见问题

1. **端口冲突**
   ```bash
   # 检查端口占用
   lsof -i :8080
   # 或使用不同端口
   docker run -d -p 9090:8080 chat-matcher:latest
   ```

2. **构建失败**
   ```bash
   # 清理 Docker 缓存
   docker system prune -a
   # 重新构建
   docker build --no-cache -t chat-matcher:latest .
   ```

3. **容器无法启动**
   ```bash
   # 查看详细日志
   docker logs chat-matcher
   # 检查容器状态
   docker inspect chat-matcher
   ```

### 日志查看

```bash
# 查看实时日志
docker logs -f chat-matcher

# 查看最近 100 行日志
docker logs --tail 100 chat-matcher

# 查看特定时间的日志
docker logs --since "2024-01-01T00:00:00" chat-matcher
```

## 🌍 生产环境部署

### 环境变量配置
```bash
# 设置时区
-e TZ=Asia/Shanghai

# 设置 Go 运行时参数
-e GOGC=20
-e GOMAXPROCS=4
```

### 反向代理配置

使用 Nginx 作为反向代理：

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://chat-matcher:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws {
        proxy_pass http://chat-matcher:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### 数据持久化

如果需要添加数据持久化：

```bash
docker run -d \
  --name chat-matcher \
  -p 8080:8080 \
  -v /host/data:/app/data \
  --restart unless-stopped \
  chat-matcher:latest
```

## 📝 注意事项

1. **数据存储**: 当前版本使用内存存储，容器重启后数据会丢失
2. **并发限制**: 默认无并发限制，生产环境建议添加适当限制
3. **安全性**: 生产环境建议配置 HTTPS 和适当的 CORS 策略
4. **监控**: 建议添加应用监控和日志收集系统

## 🤝 开发模式

开发时可以使用数据卷挂载源代码：

```bash
docker run -d \
  --name chat-matcher-dev \
  -p 8080:8080 \
  -v $(pwd):/app \
  -w /app \
  golang:1.24-alpine \
  go run main.go
```

这样可以实现代码热重载开发。