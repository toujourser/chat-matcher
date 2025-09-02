package handler

import (
	"log"
	"math/rand"
	"sync"
)

type Matcher struct {
	waitingUsers []string // 等待匹配的用户ID队列
	mu           sync.Mutex
	userStates   map[string]UserState // 用户状态map
	storage      Storage              // 添加存储接口
}

func NewMatcher(storage Storage) *Matcher {
	return &Matcher{
		userStates: make(map[string]UserState),
		storage:    storage,
	}
}

// RequestMatch 用户请求匹配
func (m *Matcher) RequestMatch(userID string) (string, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新状态
	m.userStates[userID] = StateMatching

	if len(m.waitingUsers) == 0 {
		// 队列空，加入等待
		m.waitingUsers = append(m.waitingUsers, userID)
		return "", "", false // 未匹配
	}

	// 随机选取一个等待用户
	idx := rand.Intn(len(m.waitingUsers))
	partnerID := m.waitingUsers[idx]
	if partnerID == userID {
		return "", "", false // 不能匹配自己
	}

	// 移除partner from 队列
	m.waitingUsers = append(m.waitingUsers[:idx], m.waitingUsers[idx+1:]...)

	// 更新状态
	m.userStates[userID] = StateChatting
	m.userStates[partnerID] = StateChatting

	// 记录匹配次数
	if m.storage != nil {
		if err := m.storage.IncrementMatchCount(userID); err != nil {
			log.Printf("Failed to increment match count for user %s: %v", userID, err)
		}
		if err := m.storage.IncrementMatchCount(partnerID); err != nil {
			log.Printf("Failed to increment match count for user %s: %v", partnerID, err)
		}
	}

	// 生成roomID (简单用userID+partnerID)
	roomID := userID + "-" + partnerID
	return roomID, partnerID, true
}

func (m *Matcher) CheckUserState(userID string) *UserState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if userState, ok := m.userStates[userID]; ok {
		return &userState
	}
	return nil
}

// CancelMatch 取消匹配（可选）
func (m *Matcher) CancelMatch(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, uid := range m.waitingUsers {
		if uid == userID {
			m.waitingUsers = append(m.waitingUsers[:i], m.waitingUsers[i+1:]...)
			break
		}
	}
	m.userStates[userID] = StateIdle
}
