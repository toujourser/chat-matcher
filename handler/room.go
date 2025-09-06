package handler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type RoomManager struct {
	rooms   map[string]*Room
	mu      sync.Mutex
	storage Storage // 添加存储接口
}

func NewRoomManager(storage Storage) *RoomManager {
	return &RoomManager{
		rooms:   make(map[string]*Room),
		storage: storage,
	}
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
	room.Users[user1] = &User{ID: user1, Type: UserTypeHuman}
	room.Users[user2] = &User{ID: user2, Type: UserTypeHuman}
	rm.rooms[roomID] = room

	// 创建聊天会话记录
	if rm.storage != nil {
		users := []string{user1, user2}
		if err := rm.storage.CreateChatSession(roomID, users); err != nil {
			log.Printf("Failed to create chat session in storage: %v", err)
		}
	}

	go room.Run() // 启动房间消息循环
	return room
}

// CreateAIRoom 创建包含AI用户的房间
func (rm *RoomManager) CreateAIRoom(roomID string, humanUser, aiUser string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	room := &Room{
		ID:      roomID,
		Users:   make(map[string]*User),
		MsgChan: make(chan Message),
	}
	room.Users[humanUser] = &User{ID: humanUser, Type: UserTypeHuman}
	room.Users[aiUser] = &User{ID: aiUser, Type: UserTypeAI}
	rm.rooms[roomID] = room

	// 创建聊天会话记录
	if rm.storage != nil {
		users := []string{humanUser, aiUser}
		if err := rm.storage.CreateChatSession(roomID, users); err != nil {
			log.Printf("Failed to create chat session in storage: %v", err)
		}
	}

	go room.RunWithAI(matcher.GetAIClient()) // 启动AI房间消息循环

	// AI主动发起打招呼
	go rm.sendAIGreeting(room, aiUser)

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

// RunWithAI 带AI用户的房间消息广播循环
func (r *Room) RunWithAI(aiClient *AIClient) {
	for msg := range r.MsgChan {
		// 广播消息给所有用户
		for _, user := range r.Users {
			if user.ID != msg.From && user.Conn != nil {
				if err := user.Conn.WriteJSON(msg); err != nil {
					log.Printf("Error writing to %s: %v", user.ID, err)
				}
			}
		}

		// 如果消息来自人类用户，且房间中有AI用户，则生成AI回复
		if !IsAIUser(msg.From) && aiClient != nil {
			go r.generateAIResponse(msg, aiClient)
		}
	}
}

// generateAIResponse 生成AI回复
func (r *Room) generateAIResponse(userMsg Message, aiClient *AIClient) {
	// 找到AI用户
	var aiUserID string
	for _, user := range r.Users {
		if user.Type == UserTypeAI {
			aiUserID = user.ID
			break
		}
	}

	if aiUserID == "" {
		log.Printf("No AI user found in room %s", r.ID)
		return
	}

	// 使用AI处理不同类型的消息，并提供上下文支持
	ctx := context.Background()
	var aiResponse string
	var err error

	// 使用带上下文的处理方法
	aiResponse, err = aiClient.HandleMessageWithContext(ctx, userMsg, storage, r.ID)

	if err != nil {
		log.Printf("AI response failed: %v", err)
		// 根据消息类型发送不同的错误消息
		switch userMsg.Type {
		case "image":
			aiResponse = "我看到你发送了一张图片！不过我暂时无法分析图片内容，但我很乐意和你聊聊其他话题。"
		default:
			aiResponse = "抱歉，我现在无法回复您的消息。"
		}
	}

	// 创建AI回复消息
	aiMsg := Message{
		ID:        GenerateMessageID(),
		From:      aiUserID,
		Content:   aiResponse,
		Type:      "text", // AI回复始终是文本类型
		Timestamp: time.Now(),
		RoomID:    r.ID,
	}

	// 发送AI回复给人类用户
	for _, user := range r.Users {
		if user.Type == UserTypeHuman && user.Conn != nil {
			if err := user.Conn.WriteJSON(aiMsg); err != nil {
				log.Printf("Error writing AI response to %s: %v", user.ID, err)
			}
		}
	}

	// 保存AI消息到存储
	if storage != nil {
		if err := storage.SaveMessage(aiMsg); err != nil {
			log.Printf("Failed to save AI message to storage: %v", err)
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
		log.Printf("Received message: %+v from %s\n", msg, user.ID)

		// 设置消息属性
		msg.From = user.ID
		msg.RoomID = room.ID
		msg.ID = GenerateMessageID()
		msg.Timestamp = time.Now()

		// 保存消息到存储
		if rm.storage != nil {
			if err := rm.storage.SaveMessage(msg); err != nil {
				log.Printf("Failed to save message to storage: %v", err)
			}
		}

		room.MsgChan <- msg
	}
}

// cleanupUser 清理断开用户
func (rm *RoomManager) cleanupUser(room *Room, userID string) {
	// 通知另一方（可选：发送"partner left"消息）
	for _, u := range room.Users {
		if u.ID != userID && u.Conn != nil {
			if IsAIUser(userID) {
				u.Conn.WriteJSON(Message{From: "system", Content: "AI助手已离开"})
			} else {
				u.Conn.WriteJSON(Message{From: "system", Content: "对方已经离开"})
			}
		}
	}

	// 移除房间如果空或者只剩AI用户
	rm.mu.Lock()
	delete(room.Users, userID)

	// 检查是否需要关闭房间
	shouldCloseRoom := false
	if len(room.Users) == 0 {
		shouldCloseRoom = true
	} else if len(room.Users) == 1 {
		// 如果只剩下AI用户，也关闭房间
		for _, user := range room.Users {
			if user.Type == UserTypeAI {
				shouldCloseRoom = true
				break
			}
		}
	}

	if shouldCloseRoom {
		// 结束聊天会话
		if rm.storage != nil {
			if err := rm.storage.EndChatSession(room.ID); err != nil {
				log.Printf("Failed to end chat session: %v", err)
			}
		}

		if _, ok := rm.rooms[room.ID]; ok {
			close(room.MsgChan)
		}
		delete(rm.rooms, room.ID)
	}

	// 清理用户状态（只对人类用户）
	if !IsAIUser(userID) {
		delete(matcher.userStates, userID)
	}
	rm.mu.Unlock()
}

// sendAIGreeting AI主动发起打招呼
func (rm *RoomManager) sendAIGreeting(room *Room, aiUserID string) {
	// 等待一个短暂时间，让房间和连接充分初始化
	time.Sleep(500 * time.Millisecond)

	// 检查房间是否仍然存在
	rm.mu.Lock()
	_, exists := rm.rooms[room.ID]
	rm.mu.Unlock()
	if !exists {
		return
	}

	// 生成AI打招呼消息
	var greetingContent string
	if aiClient := matcher.GetAIClient(); aiClient != nil {
		ctx := context.Background()

		aiResponse, err := aiClient.ChatResponse(ctx, RolePrompt)
		if err != nil {
			log.Printf("AI greeting generation failed: %v", err)
			greetingContent = "👋 你好！很高兴能和你聊天，有什么想聊的吗？"
		} else {
			greetingContent = aiResponse
		}
	} else {
		// 如果AI客户端不可用，使用默认打招呼
		greetingContent = "👋 你好！很高兴能和你聊天，有什么想聊的吗？"
	}

	// 创建AI打招呼消息
	greetingMsg := Message{
		ID:        GenerateMessageID(),
		From:      aiUserID,
		Content:   greetingContent,
		Type:      "text",
		Timestamp: time.Now(),
		RoomID:    room.ID,
	}

	// 保存AI消息到存储
	if rm.storage != nil {
		if err := rm.storage.SaveMessage(greetingMsg); err != nil {
			log.Printf("Failed to save AI greeting message to storage: %v", err)
		}
	}

	// 发送AI打招呼给人类用户
	for _, user := range room.Users {
		if user.Type == UserTypeHuman && user.Conn != nil {
			if err := user.Conn.WriteJSON(greetingMsg); err != nil {
				log.Printf("Error writing AI greeting to %s: %v", user.ID, err)
			}
		}
	}

	log.Printf("AI %s sent greeting in room %s: %s", aiUserID, room.ID, greetingContent)
}
