# Redis 存储功能说明

## 功能概述

本项目已成功集成Redis存储功能，支持以下特性：

1. **聊天记录存储** - 自动保存所有聊天消息
2. **用户匹配次数统计** - 记录每个用户的匹配次数和最后匹配时间
3. **聊天历史查询** - 提供API获取历史聊天记录
4. **用户统计查询** - 提供API获取用户匹配统计信息

## Redis 配置

### 环境变量配置
可以通过以下环境变量配置Redis连接：

```bash
export REDIS_ADDR="localhost:6379"      # Redis服务器地址
export REDIS_PASSWORD=""                # Redis密码（可选）
```

### 默认配置
如果未设置环境变量，将使用默认配置：
- 地址: `localhost:6379`
- 密码: 空
- 数据库: 0

## 启动 Redis

### 使用 Docker 启动 Redis
```bash
docker run -d --name redis-chat-matcher -p 6379:6379 redis:7-alpine
```

### 使用 Homebrew 安装和启动 Redis (macOS)
```bash
brew install redis
brew services start redis
```

### 使用 apt 安装 Redis (Ubuntu/Debian)
```bash
sudo apt update
sudo apt install redis-server
sudo systemctl start redis-server
sudo systemctl enable redis-server
```

## 新增的 API 接口

### 1. 获取聊天历史
**接口**: `GET /api/chat/history`

**参数**:
- `room_id` (必需): 聊天室ID
- `limit` (可选): 返回消息数量限制，默认50

**示例**:
```bash
curl "http://localhost:9093/api/chat/history?room_id=user1-user2&limit=10"
```

**响应示例**:
```json
{
  "room_id": "user1-user2",
  "messages": [
    {
      "id": "msg123456",
      "from": "user1",
      "content": "Hello!",
      "type": "text",
      "timestamp": "2025-08-27T14:20:00Z",
      "room_id": "user1-user2"
    }
  ],
  "count": 1
}
```

### 2. 获取用户匹配统计
**接口**: `GET /api/user/stats`

**参数**:
- `user_id` (必需): 用户ID

**示例**:
```bash
curl "http://localhost:9093/api/user/stats?user_id=user1"
```

**响应示例**:
```json
{
  "user_id": "user1",
  "match_count": 5,
  "last_match_at": "2025-08-27T14:20:00Z"
}
```

### 3. 获取用户参与的房间列表
**接口**: `GET /api/user/rooms`

**参数**:
- `user_id` (必需): 用户ID

**示例**:
```bash
curl "http://localhost:9093/api/user/rooms?user_id=user1"
```

**响应示例**:
```json
{
  "user_id": "user1",
  "rooms": ["user1-user2", "user1-user3"],
  "count": 2
}
```

## Redis 数据结构

### 聊天消息存储
- **Key格式**: `chat:room:{roomID}`
- **数据类型**: List
- **过期时间**: 30天
- **存储内容**: JSON格式的消息对象

### 用户匹配统计
- **Key格式**: `user:stats:{userID}`
- **数据类型**: Hash
- **字段**:
  - `user_id`: 用户ID
  - `match_count`: 匹配次数
  - `last_match_at`: 最后匹配时间

### 用户房间列表
- **Key格式**: `user:rooms:{userID}`
- **数据类型**: Set
- **过期时间**: 30天
- **存储内容**: 用户参与过的房间ID列表

### 房间信息
- **Key格式**: `room:info:{roomID}`
- **数据类型**: Hash
- **过期时间**: 30天
- **字段**:
  - `room_id`: 房间ID
  - `users`: 参与用户列表
  - `start_at`: 开始时间
  - `end_at`: 结束时间
  - `active`: 是否活跃

## 容错设计

项目在Redis不可用的情况下仍然可以正常运行：

1. **优雅降级**: 当Redis连接失败时，系统会记录错误日志但不会崩溃
2. **功能隔离**: 匹配和聊天功能不依赖Redis，只是存储功能会受影响
3. **错误处理**: 所有Redis操作都有错误处理，防止影响主要功能

## 测试示例

### 1. 完整测试流程

```bash
# 1. 启动Redis
docker run -d --name redis-test -p 6379:6379 redis:7-alpine

# 2. 启动服务器
go run main.go

# 3. 进行用户匹配
curl -X POST -H "Content-Type: application/json" \
  -d '{"user_id":"alice"}' \
  http://localhost:9093/match

curl -X POST -H "Content-Type: application/json" \
  -d '{"user_id":"bob"}' \
  http://localhost:9093/match

# 4. 查看用户统计
curl "http://localhost:9093/api/user/stats?user_id=alice"
curl "http://localhost:9093/api/user/stats?user_id=bob"

# 5. 模拟WebSocket聊天后查看历史
curl "http://localhost:9093/api/chat/history?room_id=bob-alice&limit=50"
```

### 2. 验证Redis数据

```bash
# 连接到Redis
redis-cli

# 查看用户统计
HGETALL user:stats:alice

# 查看聊天记录
LRANGE chat:room:bob-alice 0 -1

# 查看用户房间
SMEMBERS user:rooms:alice
```

## 性能优化

1. **批量操作**: 消息保存使用Pipeline提升性能
2. **过期策略**: 自动清理30天以上的数据
3. **连接池**: 使用Redis连接池避免频繁连接
4. **异步处理**: 存储操作不阻塞主业务流程

## 监控和维护

### 日志监控
服务器会记录以下关键日志：
- Redis连接状态
- 存储操作错误
- 性能相关警告

### 数据备份
建议定期备份Redis数据：
```bash
# 创建Redis快照
redis-cli BGSAVE

# 或使用Redis持久化配置
# 在redis.conf中设置：
# save 900 1
# save 300 10
# save 60 10000
```