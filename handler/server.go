package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var (
	matcher     = NewMatcher()
	roomManager = NewRoomManager()
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }, // 允许跨域
	}
)

// HandleMatch 处理匹配请求
func HandleMatch(w http.ResponseWriter, r *http.Request) {
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
