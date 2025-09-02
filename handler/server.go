package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	matcher     *Matcher
	roomManager *RoomManager
	storage     Storage
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }, // 允许跨域
	}
)

// InitializeHandlers 初始化处理器
func InitializeHandlers(storageImpl Storage) {
	storage = storageImpl
	matcher = NewMatcher(storage)
	roomManager = NewRoomManager(storage)
}

// HandleMatch 处理匹配请求
func HandleMatch(w http.ResponseWriter, r *http.Request) {
	{
		// 设置 CORS 头
		w.Header().Set("Access-Control-Allow-Origin", "*") // 允许所有域访问
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 处理预检请求（OPTIONS）
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	var req MatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := MatchResponse{Matched: false}
	// 如果用户已经在聊天状态，则直接返回匹配结果
	userState := matcher.CheckUserState(req.UserID)
	if userState != nil && *userState == StateChatting {
		for _, room := range roomManager.rooms {
			if _, ok := room.Users[req.UserID]; ok {
				resp.Matched = true
				resp.RoomID = room.ID
				resp.Partner = strings.Split(room.ID, "-")[0]
			}
		}
	} else {
		roomID, partnerID, matched := matcher.RequestMatch(req.UserID)
		resp = MatchResponse{Matched: matched, RoomID: roomID, Partner: partnerID}
		if matched {
			roomManager.CreateRoom(roomID, req.UserID, partnerID)
		}
	}

	log.Printf("用户 ID：%s, 匹配结果：%v, 房间 ID：%s, 对方 ID：%s", req.UserID, resp.Matched, resp.RoomID, resp.Partner)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleWS 处理WS连接
func HandleWS(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	userID := r.URL.Query().Get("user")
	if roomID == "" || userID == "" {
		http.Error(w, "Missing room or user", http.StatusBadRequest)
		return
	}
	log.Printf("User %s joined room %s", userID, roomID)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	roomManager.JoinRoom(roomID, userID, conn)
}

// HandleChatHistory 获取聊天历史
func HandleChatHistory(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "Missing room_id parameter", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50 // 默认限制
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if storage == nil {
		http.Error(w, "Storage not available", http.StatusServiceUnavailable)
		return
	}

	messages, err := storage.GetChatHistory(roomID, limit)
	if err != nil {
		log.Printf("Failed to get chat history: %v", err)
		http.Error(w, "Failed to get chat history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"room_id":  roomID,
		"messages": messages,
		"count":    len(messages),
	})
}

// HandleUserStats 获取用户匹配统计
func HandleUserStats(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	if storage == nil {
		http.Error(w, "Storage not available", http.StatusServiceUnavailable)
		return
	}

	stats, err := storage.GetMatchStats(userID)
	if err != nil {
		log.Printf("Failed to get user stats: %v", err)
		http.Error(w, "Failed to get user stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleUserRooms 获取用户参与的房间列表
func HandleUserRooms(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	if storage == nil {
		http.Error(w, "Storage not available", http.StatusServiceUnavailable)
		return
	}

	rooms, err := storage.GetUserChatRooms(userID)
	if err != nil {
		log.Printf("Failed to get user rooms: %v", err)
		http.Error(w, "Failed to get user rooms", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"rooms":   rooms,
		"count":   len(rooms),
	})
}

// === Gin版本的处理函数 ===

// GinHandleMatch 处理匹配请求 (Gin版本)
func GinHandleMatch(c *gin.Context) {
	var req MatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := MatchResponse{Matched: false}
	// 如果用户已经在聊天状态，则直接返回匹配结果
	userState := matcher.CheckUserState(req.UserID)
	if userState != nil && *userState == StateChatting {
		for _, room := range roomManager.rooms {
			if _, ok := room.Users[req.UserID]; ok {
				resp.Matched = true
				resp.RoomID = room.ID
				resp.Partner = strings.Split(room.ID, "-")[0]
			}
		}
	} else {
		roomID, partnerID, matched := matcher.RequestMatch(req.UserID)
		resp = MatchResponse{Matched: matched, RoomID: roomID, Partner: partnerID}
		if matched {
			roomManager.CreateRoom(roomID, req.UserID, partnerID)
		}
	}

	log.Printf("用户 ID：%s, 匹配结果：%v, 房间 ID：%s, 对方 ID：%s", req.UserID, resp.Matched, resp.RoomID, resp.Partner)

	c.JSON(http.StatusOK, resp)
}

// GinHandleWS 处理WebSocket连接 (Gin版本)
func GinHandleWS(c *gin.Context) {
	roomID := c.Query("room")
	userID := c.Query("user")
	if roomID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing room or user"})
		return
	}

	log.Printf("User %s joined room %s", userID, roomID)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade connection"})
		return
	}
	roomManager.JoinRoom(roomID, userID, conn)
}

// GinHandleChatHistory 获取聊天历史 (Gin版本)
func GinHandleChatHistory(c *gin.Context) {
	roomID := c.Query("room_id")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing room_id parameter"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	if storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Storage not available"})
		return
	}

	messages, err := storage.GetChatHistory(roomID, limit)
	if err != nil {
		log.Printf("Failed to get chat history: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room_id":  roomID,
		"messages": messages,
		"count":    len(messages),
	})
}

// GinHandleUserStats 获取用户匹配统计 (Gin版本)
func GinHandleUserStats(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id parameter"})
		return
	}

	if storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Storage not available"})
		return
	}

	stats, err := storage.GetMatchStats(userID)
	if err != nil {
		log.Printf("Failed to get user stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GinHandleUserRooms 获取用户参与的房间列表 (Gin版本)
func GinHandleUserRooms(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id parameter"})
		return
	}

	if storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Storage not available"})
		return
	}

	rooms, err := storage.GetUserChatRooms(userID)
	if err != nil {
		log.Printf("Failed to get user rooms: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user rooms"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"rooms":   rooms,
		"count":   len(rooms),
	})
}
