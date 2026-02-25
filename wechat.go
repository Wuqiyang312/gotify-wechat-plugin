package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotify/plugin-api"
)

type WeChatPlugin struct {
	userCtx    plugin.UserContext
	enabled    bool
	msgHandler plugin.MessageHandler
	storage    plugin.StorageHandler
	config     *Config
	basePath   string
	tokenCache *TokenCache
	mu         sync.RWMutex
}

type TokenCache struct {
	Token     string
	ExpiresAt time.Time
	mu        sync.RWMutex
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
}

type TemplateMessageRequest struct {
	ToUser     string                 `json:"touser"`
	TemplateID string                 `json:"template_id"`
	URL        string                 `json:"url"`
	Data       map[string]interface{} `json:"data"`
}

type WechatAPIResponse struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
	Msgid   int64  `json:"msgid"`
}

// GotifyMessage gotify 标准消息格式
type GotifyMessage struct {
	AppID    int                    `json:"appid"`
	Title    string                 `json:"title"`
	Message  string                 `json:"message" binding:"required"`
	Priority int                    `json:"priority"`
	Extras   map[string]interface{} `json:"extras"`
}

func (p *WeChatPlugin) Enable() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.config == nil {
		return fmt.Errorf("plugin not configured")
	}

	p.enabled = true
	p.tokenCache = &TokenCache{}

	log.Printf("[WeChat Plugin] Enabled for user: %s", p.userCtx.Name)
	return nil
}

func (p *WeChatPlugin) Disable() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.enabled = false
	log.Printf("[WeChat Plugin] Disabled for user: %s", p.userCtx.Name)
	return nil
}

func (p *WeChatPlugin) SetMessageHandler(h plugin.MessageHandler) {
	p.msgHandler = h
}

func (p *WeChatPlugin) SetStorageHandler(h plugin.StorageHandler) {
	p.storage = h
}

func (p *WeChatPlugin) RegisterWebhook(basePath string, router *gin.RouterGroup) {
	p.basePath = basePath

	// POST /message - gotify 标准消息格式，支持路由
	router.POST("/message", func(c *gin.Context) {
		if !p.enabled {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "plugin is disabled",
			})
			return
		}

		var msg GotifyMessage
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid request: %v", err),
			})
			return
		}

		title := msg.Title
		if title == "" {
			title = "Gotify Notification"
		}

		// 解析路由，找到目标 OpenID 列表
		openIDs := p.resolveRecipients(msg.AppID, msg.Priority)
		if len(openIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "no matching recipients, message skipped",
			})
			return
		}

		// 发送给所有匹配的接收者
		errors := p.sendToMultiple(openIDs, title, msg.Message)
		if len(errors) > 0 {
			errMsgs := make([]string, len(errors))
			for i, e := range errors {
				errMsgs[i] = e.Error()
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   fmt.Sprintf("partial failure: %d/%d failed", len(errors), len(openIDs)),
				"details": errMsgs,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message":    "sent to WeChat successfully",
			"recipients": len(openIDs),
		})
	})

	// POST /send - 向后兼容旧接口，发送给所有接收者
	router.POST("/send", func(c *gin.Context) {
		if !p.enabled {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "plugin is disabled",
			})
			return
		}

		var req struct {
			Title   string `json:"title" binding:"required"`
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid request: %v", err),
			})
			return
		}

		openIDs := p.getAllOpenIDs()
		errors := p.sendToMultiple(openIDs, req.Title, req.Content)
		if len(errors) > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to send to WeChat: %d/%d failed", len(errors), len(openIDs)),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "sent to WeChat successfully",
		})
	})

	// GET /test - 测试连接，发送给所有接收者
	router.GET("/test", func(c *gin.Context) {
		if !p.enabled {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "plugin is disabled",
			})
			return
		}

		openIDs := p.getAllOpenIDs()
		errors := p.sendToMultiple(openIDs, "Test Message", "This is a test message from Gotify WeChat Plugin")
		if len(errors) > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("test failed: %d/%d failed", len(errors), len(openIDs)),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message":    "test message sent successfully",
			"recipients": len(openIDs),
		})
	})
}

func (p *WeChatPlugin) GetDisplay(location *url.URL) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.config == nil {
		return "Plugin not configured\n\nPlease configure the plugin with your WeChat credentials."
	}

	webhookURL := &url.URL{Path: p.basePath}
	if location != nil {
		webhookURL.Scheme = location.Scheme
		webhookURL.Host = location.Host
	}

	messageURL := webhookURL.ResolveReference(&url.URL{Path: "message"})
	sendURL := webhookURL.ResolveReference(&url.URL{Path: "send"})
	testURL := webhookURL.ResolveReference(&url.URL{Path: "test"})

	status := "Disabled"
	if p.enabled {
		status = "Enabled"
	}

	// 构建接收者列表
	recipientInfo := ""
	if len(p.config.Recipients) > 0 {
		recipientInfo = "\n### Recipients\n"
		for _, r := range p.config.Recipients {
			recipientInfo += fmt.Sprintf("- **%s:** %s\n", r.Name, maskString(r.OpenID))
		}
	} else if p.config.OpenID != "" {
		recipientInfo = fmt.Sprintf("\n### Recipient\n- **OpenID:** %s\n", maskString(p.config.OpenID))
	}

	// 构建路由规则
	routeInfo := ""
	if len(p.config.Routes) > 0 {
		routeInfo = "\n### Routes\n"
		for _, route := range p.config.Routes {
			matchDesc := ""
			if len(route.Match.AppID) > 0 {
				matchDesc += fmt.Sprintf("AppID=%v ", route.Match.AppID)
			}
			if route.Match.MinPriority != nil {
				matchDesc += fmt.Sprintf("Priority>=%d ", *route.Match.MinPriority)
			}
			if matchDesc == "" {
				matchDesc = "(default/catch-all)"
			}
			recipientNames := strings.Join(route.Recipients, ", ")
			routeInfo += fmt.Sprintf("- **%s:** %s → [%s]\n", route.Name, matchDesc, recipientNames)
		}
	}

	return fmt.Sprintf(`# WeChat Template Message Pusher

**Status:** %s

## Configuration
- **AppID:** %s
- **Template ID:** %s
%s%s
## Usage

### Send via /message (Gotify Standard Format)
`+"`"+`POST %s`+"`"+`

`+"```json"+`
{
  "appid": 1,
  "title": "Message Title",
  "message": "Message Content",
  "priority": 5
}
`+"```"+`

### Send via /send (Legacy)
`+"`"+`POST %s`+"`"+`

`+"```json"+`
{
  "title": "Message Title",
  "content": "Message Content"
}
`+"```"+`

### Test Connection
Click here to test: [Send Test Message](%s)
`, status, maskString(p.config.AppID), maskString(p.config.TemplateID),
		recipientInfo, routeInfo,
		messageURL.String(), sendURL.String(), testURL.String())
}

// resolveRecipients 根据 appID 和 priority 匹配路由规则，返回目标 OpenID 列表
func (p *WeChatPlugin) resolveRecipients(appID int, priority int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 如果没有配置路由，退回到全部接收者
	if len(p.config.Routes) == 0 {
		return p.getAllOpenIDs()
	}

	// 构建 name -> openid 映射
	recipientMap := make(map[string]string)
	for _, r := range p.config.Recipients {
		recipientMap[r.Name] = r.OpenID
	}

	// 按顺序匹配路由规则
	for _, route := range p.config.Routes {
		if p.matchRoute(route, appID, priority) {
			var openIDs []string
			for _, name := range route.Recipients {
				if oid, ok := recipientMap[name]; ok {
					openIDs = append(openIDs, oid)
				}
			}
			return openIDs
		}
	}

	return nil
}

// matchRoute 检查消息是否匹配路由规则
func (p *WeChatPlugin) matchRoute(route Route, appID int, priority int) bool {
	// 检查 app_id 匹配
	if len(route.Match.AppID) > 0 {
		matched := false
		for _, id := range route.Match.AppID {
			if id == appID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查优先级匹配
	if route.Match.MinPriority != nil {
		if priority < *route.Match.MinPriority {
			return false
		}
	}

	return true
}

// getAllOpenIDs 获取所有配置的 OpenID
func (p *WeChatPlugin) getAllOpenIDs() []string {
	if len(p.config.Recipients) > 0 {
		openIDs := make([]string, 0, len(p.config.Recipients))
		for _, r := range p.config.Recipients {
			openIDs = append(openIDs, r.OpenID)
		}
		return openIDs
	}
	// 向后兼容：单 OpenID 模式
	if p.config.OpenID != "" {
		return []string{p.config.OpenID}
	}
	return nil
}

// sendToMultiple 向多个 OpenID 发送消息，返回所有错误
func (p *WeChatPlugin) sendToMultiple(openIDs []string, title, content string) []error {
	var (
		errs []error
		mu   sync.Mutex
		wg   sync.WaitGroup
	)

	for _, oid := range openIDs {
		wg.Add(1)
		go func(openID string) {
			defer wg.Done()
			if err := p.sendToWeChat(openID, title, content); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("openid %s: %w", maskString(openID), err))
				mu.Unlock()
			}
		}(oid)
	}

	wg.Wait()
	return errs
}

// sendToWeChat 向指定 OpenID 发送微信模板消息
func (p *WeChatPlugin) sendToWeChat(openID, title, content string) error {
	if p.config == nil {
		return fmt.Errorf("plugin not configured")
	}

	token, err := p.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=%s", token)

	requestData := TemplateMessageRequest{
		ToUser:     openID,
		TemplateID: p.config.TemplateID,
		URL:        p.config.JumpURL,
		Data: map[string]interface{}{
			"title": map[string]string{
				"value": title,
			},
			"content": map[string]string{
				"value": content,
			},
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post(apiURL, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp WechatAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Errcode != 0 {
		return fmt.Errorf("WeChat API error: code=%d, msg=%s", apiResp.Errcode, apiResp.Errmsg)
	}

	log.Printf("[WeChat Plugin] Message sent successfully to %s, msgid: %d", maskString(openID), apiResp.Msgid)
	return nil
}

func (p *WeChatPlugin) getAccessToken() (string, error) {
	p.tokenCache.mu.RLock()
	if p.tokenCache.Token != "" && time.Now().Before(p.tokenCache.ExpiresAt.Add(-5*time.Minute)) {
		token := p.tokenCache.Token
		p.tokenCache.mu.RUnlock()
		return token, nil
	}
	p.tokenCache.mu.RUnlock()

	p.tokenCache.mu.Lock()
	defer p.tokenCache.mu.Unlock()

	if p.tokenCache.Token != "" && time.Now().Before(p.tokenCache.ExpiresAt.Add(-5*time.Minute)) {
		return p.tokenCache.Token, nil
	}

	requestParams := map[string]interface{}{
		"grant_type": "client_credential",
		"appid":      p.config.AppID,
		"secret":     p.config.AppSecret,
	}

	jsonData, err := json.Marshal(requestParams)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post("https://api.weixin.qq.com/cgi-bin/stable_token", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp AccessTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if tokenResp.Errcode != 0 {
		return "", fmt.Errorf("WeChat API error: code=%d, msg=%s", tokenResp.Errcode, tokenResp.Errmsg)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token received")
	}

	p.tokenCache.Token = tokenResp.AccessToken
	p.tokenCache.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tokenResp.AccessToken, nil
}

func maskString(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
