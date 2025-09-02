#!/bin/bash

# Redis 存储功能测试脚本
# 使用方法: ./test_redis_storage.sh

set -e

echo "🚀 开始测试 chat-matcher Redis 存储功能"

# 检查服务器是否运行
if ! curl -s http://localhost:9093/match > /dev/null 2>&1; then
    echo "❌ 服务器未运行，请先启动 chat-matcher"
    echo "   运行命令: go run main.go"
    exit 1
fi

echo "✅ 服务器正在运行"

# 测试用户匹配
echo "📝 测试用户匹配功能..."

echo "  - 用户 alice 请求匹配"
MATCH1=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"user_id":"alice"}' \
    http://localhost:9093/match)
echo "    响应: $MATCH1"

echo "  - 用户 bob 请求匹配"
MATCH2=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"user_id":"bob"}' \
    http://localhost:9093/match)
echo "    响应: $MATCH2"

# 检查是否匹配成功
if echo "$MATCH2" | grep -q '"matched":true'; then
    echo "✅ 匹配成功"
    ROOM_ID=$(echo "$MATCH2" | grep -o '"room_id":"[^"]*"' | cut -d'"' -f4)
    echo "    房间ID: $ROOM_ID"
else
    echo "❌ 匹配失败"
fi

# 测试用户统计API
echo ""
echo "📊 测试用户统计API..."

echo "  - 查询用户 alice 的统计信息"
STATS_ALICE=$(curl -s "http://localhost:9093/api/user/stats?user_id=alice")
echo "    响应: $STATS_ALICE"

echo "  - 查询用户 bob 的统计信息"
STATS_BOB=$(curl -s "http://localhost:9093/api/user/stats?user_id=bob")
echo "    响应: $STATS_BOB"

# 测试聊天历史API
echo ""
echo "💬 测试聊天历史API..."

if [ ! -z "$ROOM_ID" ]; then
    echo "  - 查询房间 $ROOM_ID 的聊天历史"
    HISTORY=$(curl -s "http://localhost:9093/api/chat/history?room_id=$ROOM_ID&limit=10")
    echo "    响应: $HISTORY"
else
    echo "  - 跳过聊天历史测试（无有效房间ID）"
fi

# 测试用户房间列表API
echo ""
echo "🏠 测试用户房间列表API..."

echo "  - 查询用户 alice 的房间列表"
ROOMS_ALICE=$(curl -s "http://localhost:9093/api/user/rooms?user_id=alice")
echo "    响应: $ROOMS_ALICE"

echo "  - 查询用户 bob 的房间列表"
ROOMS_BOB=$(curl -s "http://localhost:9093/api/user/rooms?user_id=bob")
echo "    响应: $ROOMS_BOB"

# 检查Redis连接状态
echo ""
echo "🔗 检查Redis连接状态..."

if command -v redis-cli > /dev/null 2>&1; then
    if redis-cli ping > /dev/null 2>&1; then
        echo "✅ Redis 连接正常"
        
        echo "  - 检查Redis中的数据:"
        echo "    用户统计数据:"
        redis-cli HGETALL user:stats:alice 2>/dev/null || echo "    (无数据或Redis未连接)"
        redis-cli HGETALL user:stats:bob 2>/dev/null || echo "    (无数据或Redis未连接)"
        
        echo "    房间数据:"
        if [ ! -z "$ROOM_ID" ]; then
            redis-cli LLEN chat:room:$ROOM_ID 2>/dev/null || echo "    (无消息数据)"
        fi
    else
        echo "⚠️  Redis 未运行或连接失败"
        echo "   要启动Redis: docker run -d --name redis-test -p 6379:6379 redis:7-alpine"
    fi
else
    echo "⚠️  redis-cli 未安装，无法检查Redis状态"
fi

echo ""
echo "🎉 测试完成！"
echo ""
echo "💡 提示:"
echo "   - 如果看到 'Storage not available' 或 'Failed to get...'，表示Redis未连接"
echo "   - 启动Redis后，统计和历史记录功能将正常工作"
echo "   - 即使没有Redis，基本的匹配和聊天功能仍然可用"