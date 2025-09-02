package handler

import (
	"encoding/json"
	"fmt"
	"time"
)

// Storage 存储接口
type Storage interface {
	// 聊天记录相关
	SaveMessage(message Message) error
	GetChatHistory(roomID string, limit int) ([]Message, error)
	GetUserChatRooms(userID string) ([]string, error)

	// 用户匹配统计相关
	IncrementMatchCount(userID string) error
	GetMatchStats(userID string) (*UserMatchStats, error)
	GetAllUserStats() ([]UserMatchStats, error)

	// 房间相关
	CreateChatSession(roomID string, users []string) error
	EndChatSession(roomID string) error
}

// RedisStorage Redis存储实现
type RedisStorage struct {
	redis *RedisManager
}

// NewRedisStorage 创建Redis存储实例
func NewRedisStorage(redisManager *RedisManager) Storage {
	return &RedisStorage{
		redis: redisManager,
	}
}

// Redis key 生成函数
func (rs *RedisStorage) getChatHistoryKey(roomID string) string {
	return fmt.Sprintf("chat:room:%s", roomID)
}

func (rs *RedisStorage) getUserRoomsKey(userID string) string {
	return fmt.Sprintf("user:rooms:%s", userID)
}

func (rs *RedisStorage) getMatchStatsKey(userID string) string {
	return fmt.Sprintf("user:stats:%s", userID)
}

func (rs *RedisStorage) getRoomInfoKey(roomID string) string {
	return fmt.Sprintf("room:info:%s", roomID)
}

// SaveMessage 保存消息到Redis
func (rs *RedisStorage) SaveMessage(message Message) error {
	if !rs.redis.IsConnected() {
		return fmt.Errorf("Redis not connected")
	}

	// 设置消息时间戳
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// 序列化消息
	msgData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// 保存到聊天历史列表
	chatKey := rs.getChatHistoryKey(message.RoomID)
	err = rs.redis.client.LPush(rs.redis.ctx, chatKey, msgData).Err()
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// 设置过期时间 (30天)
	rs.redis.client.Expire(rs.redis.ctx, chatKey, 30*24*time.Hour)

	// 为发送者添加房间记录
	userRoomsKey := rs.getUserRoomsKey(message.From)
	rs.redis.client.SAdd(rs.redis.ctx, userRoomsKey, message.RoomID)
	rs.redis.client.Expire(rs.redis.ctx, userRoomsKey, 30*24*time.Hour)

	return nil
}

// GetChatHistory 获取聊天历史
func (rs *RedisStorage) GetChatHistory(roomID string, limit int) ([]Message, error) {
	if !rs.redis.IsConnected() {
		return nil, fmt.Errorf("Redis not connected")
	}

	if limit <= 0 {
		limit = 100 // 默认限制100条
	}

	chatKey := rs.getChatHistoryKey(roomID)
	result, err := rs.redis.client.LRange(rs.redis.ctx, chatKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get chat history: %w", err)
	}

	messages := make([]Message, 0, len(result))
	for i := len(result) - 1; i >= 0; i-- { // 反向遍历以获得正确的时间顺序
		var msg Message
		if err := json.Unmarshal([]byte(result[i]), &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// GetUserChatRooms 获取用户参与的聊天室列表
func (rs *RedisStorage) GetUserChatRooms(userID string) ([]string, error) {
	if !rs.redis.IsConnected() {
		return nil, fmt.Errorf("Redis not connected")
	}

	userRoomsKey := rs.getUserRoomsKey(userID)
	rooms, err := rs.redis.client.SMembers(rs.redis.ctx, userRoomsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user rooms: %w", err)
	}

	return rooms, nil
}

// IncrementMatchCount 增加用户匹配次数
func (rs *RedisStorage) IncrementMatchCount(userID string) error {
	if !rs.redis.IsConnected() {
		return fmt.Errorf("Redis not connected")
	}

	statsKey := rs.getMatchStatsKey(userID)

	// 增加匹配次数
	err := rs.redis.client.HIncrBy(rs.redis.ctx, statsKey, "match_count", 1).Err()
	if err != nil {
		return fmt.Errorf("failed to increment match count: %w", err)
	}

	// 更新最后匹配时间
	now := time.Now().Format(time.RFC3339)
	err = rs.redis.client.HSet(rs.redis.ctx, statsKey, "last_match_at", now).Err()
	if err != nil {
		return fmt.Errorf("failed to update last match time: %w", err)
	}

	// 设置用户ID
	rs.redis.client.HSet(rs.redis.ctx, statsKey, "user_id", userID)

	return nil
}

// GetMatchStats 获取用户匹配统计
func (rs *RedisStorage) GetMatchStats(userID string) (*UserMatchStats, error) {
	if !rs.redis.IsConnected() {
		return nil, fmt.Errorf("Redis not connected")
	}

	statsKey := rs.getMatchStatsKey(userID)
	result, err := rs.redis.client.HGetAll(rs.redis.ctx, statsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get match stats: %w", err)
	}

	if len(result) == 0 {
		// 返回默认统计信息
		return &UserMatchStats{
			UserID:     userID,
			MatchCount: 0,
		}, nil
	}

	stats := &UserMatchStats{
		UserID: userID,
	}

	if countStr, ok := result["match_count"]; ok {
		if count, err := rs.redis.client.Get(rs.redis.ctx, "").Int(); err == nil {
			stats.MatchCount = count
		} else {
			// 手动解析
			fmt.Sscanf(countStr, "%d", &stats.MatchCount)
		}
	}

	if lastMatch, ok := result["last_match_at"]; ok {
		stats.LastMatchAt = lastMatch
	}

	return stats, nil
}

// GetAllUserStats 获取所有用户统计（用于管理和调试）
func (rs *RedisStorage) GetAllUserStats() ([]UserMatchStats, error) {
	if !rs.redis.IsConnected() {
		return nil, fmt.Errorf("Redis not connected")
	}

	// 使用SCAN来查找所有匹配统计key
	pattern := "user:stats:*"
	keys, err := rs.redis.client.Keys(rs.redis.ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to scan user stats keys: %w", err)
	}

	stats := make([]UserMatchStats, 0, len(keys))
	for _, key := range keys {
		// 从key中提取userID
		userID := key[len("user:stats:"):]

		userStats, err := rs.GetMatchStats(userID)
		if err == nil && userStats != nil {
			stats = append(stats, *userStats)
		}
	}

	return stats, nil
}

// CreateChatSession 创建聊天会话
func (rs *RedisStorage) CreateChatSession(roomID string, users []string) error {
	if !rs.redis.IsConnected() {
		return fmt.Errorf("Redis not connected")
	}

	roomKey := rs.getRoomInfoKey(roomID)

	// 保存房间信息
	roomInfo := map[string]interface{}{
		"room_id":  roomID,
		"users":    fmt.Sprintf("%v", users),
		"start_at": time.Now().Format(time.RFC3339),
		"active":   "true",
	}

	err := rs.redis.client.HMSet(rs.redis.ctx, roomKey, roomInfo).Err()
	if err != nil {
		return fmt.Errorf("failed to create chat session: %w", err)
	}

	// 设置过期时间
	rs.redis.client.Expire(rs.redis.ctx, roomKey, 30*24*time.Hour)

	// 为所有用户添加房间记录
	for _, userID := range users {
		userRoomsKey := rs.getUserRoomsKey(userID)
		rs.redis.client.SAdd(rs.redis.ctx, userRoomsKey, roomID)
		rs.redis.client.Expire(rs.redis.ctx, userRoomsKey, 30*24*time.Hour)
	}

	return nil
}

// EndChatSession 结束聊天会话
func (rs *RedisStorage) EndChatSession(roomID string) error {
	if !rs.redis.IsConnected() {
		return fmt.Errorf("Redis not connected")
	}

	roomKey := rs.getRoomInfoKey(roomID)

	// 更新结束时间和状态
	updates := map[string]interface{}{
		"end_at": time.Now().Format(time.RFC3339),
		"active": "false",
	}

	return rs.redis.client.HMSet(rs.redis.ctx, roomKey, updates).Err()
}
