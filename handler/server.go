package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// MatchHandle 处理匹配请求 (Gin版本)
func MatchHandle(c *gin.Context) {
	var req MatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()

	interval := 1 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var resp MatchResponse
	for {
		resp = match(req)
		select {
		case <-ctx.Done():
			log.Println("超时或被取消，退出")
			c.JSON(http.StatusOK, resp)
			return

		case <-ticker.C:
			if resp.Matched {
				log.Printf("用户 ID：%s, 匹配结果：%v, 房间 ID：%s, 对方 ID：%s", req.UserID, resp.Matched, resp.RoomID, resp.Partner)
				c.JSON(http.StatusOK, resp)
				return
			}
		}
	}
}

func match(req MatchRequest) MatchResponse {
	resp := MatchResponse{Matched: false}
	// 如果用户已经在聊天状态，则直接返回匹配结果
	userState := matcher.CheckUserState(req.UserID)
	if userState != nil && *userState == StateChatting {
		for _, room := range roomManager.rooms {
			for userId, user := range room.Users {
				if userId == req.UserID {
					resp.Matched = true
					resp.RoomID = room.ID
					resp.Partner = strings.Split(room.ID, "-")[0]
				} else {
					resp.Partner = user.ID
				}
			}
		}
	} else {
		roomID, partnerID, matched := matcher.RequestMatch(req.UserID)
		resp = MatchResponse{Matched: matched, RoomID: roomID, Partner: partnerID}
		if matched {
			roomManager.CreateRoom(roomID, req.UserID, partnerID)
		}
	}
	return resp
}

// WSHandle 处理WebSocket连接 (Gin版本)
func WSHandle(c *gin.Context) {
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

// ChatHistoryHandle 获取聊天历史 (Gin版本)
func ChatHistoryHandle(c *gin.Context) {
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

// UserStatsHandle 获取用户匹配统计 (Gin版本)
func UserStatsHandle(c *gin.Context) {
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

// UserRoomsHandle 获取用户参与的房间列表 (Gin版本)
func UserRoomsHandle(c *gin.Context) {
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
