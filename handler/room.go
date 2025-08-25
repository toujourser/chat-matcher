package handler

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type RoomManager struct {
	rooms map[string]*Room
	mu    sync.Mutex
}

func NewRoomManager() *RoomManager {
	return &RoomManager{rooms: make(map[string]*Room)}
}

// CreateRoom 创建房间
func (rm *RoomManager) CreateRoom(roomID string, user1, user2 string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	room := &Room{
		ID:      roomID,
		Users:   make(map[string]*User),
		MsgChan: make(chan Message),
	}
	room.Users[user1] = &User{ID: user1}
	room.Users[user2] = &User{ID: user2}
	rm.rooms[roomID] = room
	go room.Run() // 启动房间消息循环
	return room
}

// Run 房间消息广播循环
func (r *Room) Run() {
	for msg := range r.MsgChan {
		for _, user := range r.Users {
			if user.ID != msg.From && user.Conn != nil {
				if err := user.Conn.WriteJSON(msg); err != nil {
					log.Printf("Error writing to %s: %v", user.ID, err)
				}
			}
		}
	}
}

// JoinRoom 用户加入WS
func (rm *RoomManager) JoinRoom(roomID, userID string, conn *websocket.Conn) {
	rm.mu.Lock()
	room, ok := rm.rooms[roomID]
	rm.mu.Unlock()
	if !ok {
		return
	}
	user, ok := room.Users[userID]
	if ok {
		user.Conn = conn
	}
	go rm.handleMessages(room, user)
}

// handleMessages 处理用户消息
func (rm *RoomManager) handleMessages(room *Room, user *User) {
	defer func() {
		user.Conn.Close()
		rm.cleanupUser(room, user.ID)
	}()
	for {
		var msg Message
		err := user.Conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}
		msg.From = user.ID
		room.MsgChan <- msg
	}
}

// cleanupUser 清理断开用户
func (rm *RoomManager) cleanupUser(room *Room, userID string) {
	// 通知另一方（可选：发送"partner left"消息）
	for _, u := range room.Users {
		if u.ID != userID && u.Conn != nil {
			u.Conn.WriteJSON(Message{From: "system", Content: "对方已经离开"})
		}
	}
	// 移除房间如果空
	rm.mu.Lock()
	delete(room.Users, userID)
	if len(room.Users) == 0 {
		close(room.MsgChan)
		delete(rm.rooms, room.ID)
	}
	rm.mu.Unlock()
}
