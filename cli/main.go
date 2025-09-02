package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

// 应用状态
type AppState int

const (
	StateMenu AppState = iota
	StateMatching
	StateChatting
	StateHelp
)

// 消息类型
type Message struct {
	From    string `json:"from"`
	Content string `json:"content"`
}

// 匹配请求
type MatchRequest struct {
	UserID string `json:"user_id"`
}

// 匹配响应
type MatchResponse struct {
	Matched   bool   `json:"matched"`
	RoomID    string `json:"room_id"`
	PartnerID string `json:"partner_id"`
}

// 自定义消息类型
type matchSuccessMsg struct {
	roomID    string
	partnerID string
}

type matchFailMsg struct{}
type messageReceivedMsg Message
type wsConnectedMsg struct {
	conn *websocket.Conn
}
type wsErrorMsg struct {
	err error
}

// 主模型
type model struct {
	state        AppState
	userID       string
	roomID       string
	partnerID    string
	conn         *websocket.Conn
	messages     []Message
	input        textinput.Model
	viewport     viewport.Model
	menuChoice   int
	matchRetries int
	ready        bool
	width        int
	height       int
}

// 样式定义
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			PaddingLeft(2)

	menuStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingRight(2)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD")).
			PaddingLeft(2).
			PaddingRight(2)

	messageStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			PaddingLeft(2)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			PaddingLeft(2)
)

func initialModel() model {
	// 创建文本输入框
	ti := textinput.New()
	ti.Placeholder = "输入您的消息..."
	ti.CharLimit = 256
	ti.Width = 50

	// 创建视口
	vp := viewport.New(78, 20)
	vp.YPosition = 1

	return model{
		state:      StateMenu,
		userID:     fmt.Sprintf("user_%d", time.Now().Unix()),
		input:      ti,
		viewport:   vp,
		menuChoice: 0,
		messages:   []Message{},
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
		m.input.Width = msg.Width - 8
		m.ready = true

	case tea.KeyMsg:
		switch m.state {
		case StateMenu:
			return m.handleMenuKeys(msg)
		case StateMatching:
			return m.handleMatchingKeys(msg)
		case StateChatting:
			return m.handleChattingKeys(msg)
		case StateHelp:
			return m.handleHelpKeys(msg)
		}

	case matchSuccessMsg:
		m.roomID = msg.roomID
		m.partnerID = msg.partnerID
		m.state = StateChatting
		m.input.Focus() // 激活输入框焦点
		m.messages = []Message{
			{From: "系统", Content: fmt.Sprintf("成功匹配到聊天伙伴: %s", msg.partnerID)},
		}
		m.updateViewport()
		return m, m.connectWebSocket

	case matchFailMsg:
		m.matchRetries++
		if m.matchRetries >= 30 {
			m.state = StateMenu
			m.matchRetries = 0
			return m, tea.Sequence(
				tea.Printf("匹配失败次数过多，返回主菜单"),
				tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
					return nil
				}),
			)
		}
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return m.requestMatch()
		})

	case wsConnectedMsg:
		m.conn = msg.conn
		m.input.Focus() // 确保输入框有焦点
		m.messages = append(m.messages, Message{
			From:    "系统",
			Content: "WebSocket 连接已建立，开始聊天吧！",
		})
		m.updateViewport()
		// 使用单次消息监听
		return m, func() tea.Msg {
			return m.listenForMessages()
		}

	case messageReceivedMsg:
		m.messages = append(m.messages, Message(msg))
		m.updateViewport()
		// 继续监听下一条消息
		if m.state == StateChatting && m.conn != nil {
			return m, func() tea.Msg {
				return m.listenForMessages()
			}
		}

	case wsErrorMsg:
		m.messages = append(m.messages, Message{
			From:    "系统",
			Content: fmt.Sprintf("连接错误: %v", msg.err),
		})
		m.state = StateMenu
		m.conn = nil
		m.updateViewport()
	}

	// 更新视口
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  正在初始化..."
	}

	switch m.state {
	case StateMenu:
		return m.viewMenu()
	case StateMatching:
		return m.viewMatching()
	case StateChatting:
		return m.viewChatting()
	case StateHelp:
		return m.viewHelp()
	default:
		return "未知状态"
	}
}

func (m model) viewMenu() string {
	s := titleStyle.Render("🚀 终端聊天客户端")
	s += "\n\n"
	s += menuStyle.Render(fmt.Sprintf("用户ID: %s", m.userID))
	s += "\n\n"
	s += menuStyle.Render("请选择操作:")
	s += "\n\n"

	menuItems := []string{
		"1. 开始匹配",
		"2. 退出程序",
		"3. 帮助信息",
	}

	for i, item := range menuItems {
		if i == m.menuChoice {
			s += selectedStyle.Render(item) + "\n"
		} else {
			s += normalStyle.Render(item) + "\n"
		}
	}

	s += "\n"
	s += menuStyle.Render("使用 ↑/↓ 选择，回车确认")

	return s
}

func (m model) viewMatching() string {
	s := titleStyle.Render("🔍 正在匹配...")
	s += "\n\n"
	s += systemStyle.Render(fmt.Sprintf("用户ID: %s", m.userID))
	s += "\n"
	s += systemStyle.Render(fmt.Sprintf("匹配尝试次数: %d/30", m.matchRetries))
	s += "\n\n"
	s += messageStyle.Render("正在寻找聊天伙伴，请稍候...")
	s += "\n\n"
	s += menuStyle.Render("按 'q' 或 ESC 返回主菜单")

	return s
}

func (m model) viewChatting() string {
	s := titleStyle.Render(fmt.Sprintf("💬 聊天室: %s", m.roomID))
	s += "\n"
	s += systemStyle.Render(fmt.Sprintf("聊天伙伴: %s", m.partnerID))
	s += "\n\n"

	// 显示聊天记录
	s += m.viewport.View()
	s += "\n"

	// 显示输入框
	s += lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render(m.input.View())
	s += "\n"
	s += menuStyle.Render("按 ESC 返回主菜单，输入 /quit 退出聊天室")

	return s
}

func (m model) viewHelp() string {
	s := titleStyle.Render("📖 帮助信息")
	s += "\n\n"
	s += menuStyle.Render("功能说明:")
	s += "\n"
	s += normalStyle.Render("• 匹配: 与其他在线用户进行随机匹配")
	s += "\n"
	s += normalStyle.Render("• 聊天: 与匹配成功的用户进行实时聊天")
	s += "\n"
	s += normalStyle.Render("• 退出: 退出程序")
	s += "\n\n"
	s += menuStyle.Render("操作说明:")
	s += "\n"
	s += normalStyle.Render("• ↑/↓: 在菜单中选择选项")
	s += "\n"
	s += normalStyle.Render("• 回车: 确认选择")
	s += "\n"
	s += normalStyle.Render("• ESC: 返回上级菜单")
	s += "\n"
	s += normalStyle.Render("• q: 退出当前操作")
	s += "\n"
	s += normalStyle.Render("• /quit: 在聊天中输入此命令可退出聊天室")
	s += "\n\n"
	s += menuStyle.Render("服务器信息:")
	s += "\n"
	s += normalStyle.Render("• 匹配接口: POST http://127.0.0.1:9093/api/match")
	s += "\n"
	s += normalStyle.Render("• WebSocket: ws://127.0.0.1:9093/api/ws")
	s += "\n\n"
	s += menuStyle.Render("按任意键返回主菜单")

	return s
}

// 处理主菜单按键
func (m model) handleMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.menuChoice > 0 {
			m.menuChoice--
		}
	case "down", "j":
		if m.menuChoice < 2 {
			m.menuChoice++
		}
	case "enter":
		switch m.menuChoice {
		case 0: // 开始匹配
			m.state = StateMatching
			m.matchRetries = 0
			return m, func() tea.Msg {
				return m.requestMatch()
			}
		case 1: // 退出程序
			return m, tea.Quit
		case 2: // 帮助信息
			m.state = StateHelp
		}
	}
	return m, nil
}

// 处理匹配中按键
func (m model) handleMatchingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.state = StateMenu
		m.matchRetries = 0
	}
	return m, nil
}

// 处理聊天中按键
func (m model) handleChattingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.conn != nil {
			m.conn.Close()
			m.conn = nil
		}
		m.state = StateMenu
		m.input.SetValue("")
		m.input.Blur() // 取消输入框焦点
		return m, nil
	case "enter":
		if m.conn != nil && strings.TrimSpace(m.input.Value()) != "" {
			inputContent := strings.TrimSpace(m.input.Value())

			// 检查是否为退出命令
			if inputContent == "/quit" {
				// 发送退出通知消息
				m.messages = append(m.messages, Message{
					From:    "系统",
					Content: "您已退出聊天室",
				})
				m.updateViewport()

				// 关闭WebSocket连接
				if m.conn != nil {
					m.conn.Close()
					m.conn = nil
				}
				m.state = StateMenu
				m.input.SetValue("")
				m.input.Blur() // 取消输入框焦点
				return m, nil
			}

			message := Message{
				From:    m.userID,
				Content: inputContent,
			}

			// 发送消息
			if err := m.conn.WriteJSON(message); err != nil {
				m.messages = append(m.messages, Message{
					From:    "系统",
					Content: "消息发送失败: " + err.Error(),
				})
				m.conn = nil // 清除断开的连接
			} else {
				// 添加到本地消息列表
				m.messages = append(m.messages, message)
			}
			m.input.SetValue("")
			m.updateViewport()
		}
		return m, nil
	default:
		// 对于其他所有按键，让输入框处理
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// 处理帮助页面按键
func (m model) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.state = StateMenu
	}
	return m, nil
}

// 请求匹配
func (m model) requestMatch() tea.Msg {
	req := MatchRequest{UserID: m.userID}
	reqBody, _ := json.Marshal(req)

	resp, err := http.Post("http://127.0.0.1:9093/api/match", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return matchFailMsg{}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return matchFailMsg{}
	}

	var matchResp MatchResponse
	if err := json.Unmarshal(body, &matchResp); err != nil {
		return matchFailMsg{}
	}

	if matchResp.Matched {
		return matchSuccessMsg{
			roomID:    matchResp.RoomID,
			partnerID: matchResp.PartnerID,
		}
	}

	return matchFailMsg{}
}

// 连接WebSocket
func (m model) connectWebSocket() tea.Msg {
	u := url.URL{Scheme: "ws", Host: "127.0.0.1:9093", Path: "/api/ws"}
	q := u.Query()
	q.Set("room", m.roomID)
	q.Set("user", m.userID)
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return wsErrorMsg{err: err}
	}

	return wsConnectedMsg{conn: conn}
}

// 监听WebSocket消息
func (m model) listenForMessages() tea.Msg {
	if m.conn == nil || m.state != StateChatting {
		return nil // 如果连接不存在或不在聊天状态，停止监听
	}

	var message Message
	err := m.conn.ReadJSON(&message)
	if err != nil {
		// 连接错误
		return wsErrorMsg{err: err}
	}

	return messageReceivedMsg(message)
}

// 更新视口内容
func (m *model) updateViewport() {
	var content strings.Builder
	for _, msg := range m.messages {
		if msg.From == "系统" {
			content.WriteString(systemStyle.Render(fmt.Sprintf("[%s] %s", msg.From, msg.Content)))
		} else if msg.From == m.userID {
			content.WriteString(messageStyle.Render(fmt.Sprintf("[我] %s", msg.Content)))
		} else {
			content.WriteString(messageStyle.Render(fmt.Sprintf("[%s] %s", msg.From, msg.Content)))
		}
		content.WriteString("\n")
	}
	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

func main() {
	f, err := os.OpenFile("chat-client.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("程序运行出错: %v", err)
		os.Exit(1)
	}
}
