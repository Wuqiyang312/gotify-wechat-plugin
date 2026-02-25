package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// GotifyMessage 对应 Gotify API 的 Message 模型
type GotifyMessage struct {
	ID       int64                  `json:"id"`
	AppID    int64                  `json:"appid"`
	Title    string                 `json:"title"`
	Message  string                 `json:"message"`
	Priority int                    `json:"priority"`
	Date     string                 `json:"date"`
	Extras   map[string]interface{} `json:"extras"`
}

// MessageRouter 消息路由器，根据配置的路径规则过滤消息
type MessageRouter struct {
	appIDs   map[int64]bool
	allowAll bool
}

// 从路径末尾提取数字的正则
var pathIDRegex = regexp.MustCompile(`(\d+)$`)

// NewMessageRouter 解析路径规则，构建路由器
func NewMessageRouter(routes []MessageRoute) *MessageRouter {
	r := &MessageRouter{
		appIDs: make(map[int64]bool),
	}

	for _, route := range routes {
		path := strings.TrimSpace(route.Path)
		if path == "*" {
			r.allowAll = true
			return r
		}

		// 从路径末尾提取数字作为 appid
		matches := pathIDRegex.FindStringSubmatch(path)
		if len(matches) == 2 {
			if id, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				r.appIDs[id] = true
			}
		}
	}

	return r
}

// Match 判断消息是否匹配路由规则
func (r *MessageRouter) Match(msg GotifyMessage) bool {
	if r.allowAll {
		return true
	}
	return r.appIDs[msg.AppID]
}

// StreamListener WebSocket 流监听器
type StreamListener struct {
	plugin *WeChatPlugin
	conn   *websocket.Conn
	router *MessageRouter
	stopCh chan struct{}
	done   chan struct{}
	mu     sync.Mutex
}

// NewStreamListener 创建流监听器
func NewStreamListener(p *WeChatPlugin) *StreamListener {
	return &StreamListener{
		plugin: p,
		router: NewMessageRouter(p.config.MessageRoutes),
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start 启动监听（在 goroutine 中运行，含自动重连）
func (s *StreamListener) Start() {
	defer close(s.done)

	backoff := time.Second
	maxBackoff := 2 * time.Minute

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		err := s.connectAndListen()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
			}

			log.Printf("[WeChat Plugin] Stream disconnected: %v, reconnecting in %v", err, backoff)
			s.plugin.msgMgr.NotifyError("Stream 连接断开", []error{err}, 1)

			select {
			case <-time.After(backoff):
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			case <-s.stopCh:
				return
			}
		}
	}
}

// Stop 停止监听
func (s *StreamListener) Stop() {
	close(s.stopCh)

	s.mu.Lock()
	if s.conn != nil {
		s.conn.Close()
	}
	s.mu.Unlock()

	<-s.done
}

// Connected 返回当前是否已连接
func (s *StreamListener) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

// resolveGotifyURL 解析 Gotify WebSocket URL（自动发现或手动配置）
func (s *StreamListener) resolveGotifyURL() (string, error) {
	baseURL := s.plugin.config.GotifyURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost"
	}

	// 确保有 scheme
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid gotify_url: %w", err)
	}

	// HTTP -> WS, HTTPS -> WSS
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	default:
		parsed.Scheme = "ws"
	}

	// 构建 /stream 路径
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/stream"

	// 添加 client token
	q := parsed.Query()
	q.Set("token", s.plugin.config.ClientToken)
	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}

// connectAndListen 建立连接并监听消息，返回错误时触发重连
func (s *StreamListener) connectAndListen() error {
	wsURL, err := s.resolveGotifyURL()
	if err != nil {
		return err
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.conn = nil
		conn.Close()
		s.mu.Unlock()
	}()

	log.Printf("[WeChat Plugin] Connected to Gotify stream")

	for {
		select {
		case <-s.stopCh:
			return nil
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message failed: %w", err)
		}

		var msg GotifyMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("[WeChat Plugin] Failed to parse stream message: %v", err)
			continue
		}

		if s.router.Match(msg) {
			go s.forwardToWeChat(msg)
		}
	}
}

// forwardToWeChat 将 Gotify 消息转发到微信
func (s *StreamListener) forwardToWeChat(msg GotifyMessage) {
	title := msg.Title
	if title == "" {
		title = "Gotify Notification"
	}

	content := msg.Message
	if content == "" {
		content = "(empty message)"
	}

	openIDs := s.plugin.getAllOpenIDs()
	if len(openIDs) == 0 {
		log.Printf("[WeChat Plugin] No recipients configured, skipping message %d", msg.ID)
		return
	}

	s.plugin.sendToMultiple(openIDs, title, content)
}
