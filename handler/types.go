package handler

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gorilla/websocket"
)

// UserMatchStats 用户匹配统计
type UserMatchStats struct {
	UserID      string `json:"user_id"`       // 用户ID
	MatchCount  int    `json:"match_count"`   // 匹配次数
	LastMatchAt string `json:"last_match_at"` // 最后匹配时间
}

// ChatHistory 聊天历史
type ChatHistory struct {
	RoomID   string    `json:"room_id"`  // 房间ID
	Messages []Message `json:"messages"` // 消息列表
	Users    []string  `json:"users"`    // 参与用户列表
	StartAt  time.Time `json:"start_at"` // 聊天开始时间
	EndAt    time.Time `json:"end_at"`   // 聊天结束时间
}

// GenerateMessageID 生成唯一消息ID
func GenerateMessageID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateAIUserID 生成AI用户ID
func GenerateAIUserID() string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return "ai_" + hex.EncodeToString(bytes)
}

// IsAIUser 检查是否为AI用户
func IsAIUser(userID string) bool {
	return len(userID) > 3 && userID[:3] == "ai_"
}

// UserState 用户状态
type UserState string

const (
	StateIdle     UserState = "idle"
	StateMatching UserState = "matching"
	StateChatting UserState = "chatting"
)

// UserType 用户类型
type UserType string

const (
	UserTypeHuman UserType = "human" // 真实用户
	UserTypeAI    UserType = "ai"    // AI用户
)

// User 用户信息
type User struct {
	ID    string
	Type  UserType // 用户类型
	State UserState
	Conn  *websocket.Conn // WS连接（聊天时使用）
}

// MatchRequest 匹配请求
type MatchRequest struct {
	UserID string `json:"user_id"`
}

// MatchResponse 匹配响应
type MatchResponse struct {
	Matched bool   `json:"matched"`
	RoomID  string `json:"room_id"`
	Partner string `json:"partner_id"` // 可选，返回对方ID
}

// Room 聊天室
type Room struct {
	ID      string
	Users   map[string]*User
	MsgChan chan Message
}

// Message 消息结构体
type Message struct {
	ID        string    `json:"id,omitempty"`        // 消息唯一ID
	From      string    `json:"from"`                // 发送者用户ID
	Content   string    `json:"content"`             // 消息内容
	Type      string    `json:"type"`                // text/image/audio/video
	Timestamp time.Time `json:"timestamp,omitempty"` // 消息时间戳
	RoomID    string    `json:"room_id,omitempty"`   // 聊天室ID
}
