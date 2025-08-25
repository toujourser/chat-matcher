package handler

import (
	"github.com/gorilla/websocket"
)

// UserState 用户状态
type UserState string

const (
	StateIdle     UserState = "idle"
	StateMatching UserState = "matching"
	StateChatting UserState = "chatting"
)

// User 用户信息
type User struct {
	ID    string
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
	From    string `json:"from"`
	Content string `json:"content"`
	Type    string `json:"type"` // text/image/audio/video
}
