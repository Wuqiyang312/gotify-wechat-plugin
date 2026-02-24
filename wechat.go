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

		if err := p.sendToWeChat(req.Title, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to send to WeChat: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "sent to WeChat successfully",
		})
	})

	router.GET("/test", func(c *gin.Context) {
		if !p.enabled {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "plugin is disabled",
			})
			return
		}

		err := p.sendToWeChat("Test Message", "This is a test message from Gotify WeChat Plugin")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("test failed: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "test message sent successfully",
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
	webhookURL = webhookURL.ResolveReference(&url.URL{Path: "send"})

	testURL := &url.URL{Path: p.basePath}
	if location != nil {
		testURL.Scheme = location.Scheme
		testURL.Host = location.Host
	}
	testURL = testURL.ResolveReference(&url.URL{Path: "test"})

	status := "Disabled"
	if p.enabled {
		status = "Enabled"
	}

	return fmt.Sprintf(`# WeChat Template Message Pusher

**Status:** %s

## Configuration
- **AppID:** %s
- **OpenID:** %s
- **Template ID:** %s

## Usage

### Send via Webhook
Send POST request to:
`+"`"+`
%s
`+"`"+`

**Request Body:**
`+"```json"+`
{
  "title": "Message Title",
  "content": "Message Content"
}
`+"```"+`

### Test Connection
Click here to test: [Send Test Message](%s)

## Notes
- Messages will be forwarded to WeChat using template messages
- Make sure your WeChat service account has the template message permission
- The template should have fields: title, content
`, status, maskString(p.config.AppID), maskString(p.config.OpenID), maskString(p.config.TemplateID), webhookURL.String(), testURL.String())
}

func (p *WeChatPlugin) sendToWeChat(title, content string) error {
	if p.config == nil {
		return fmt.Errorf("plugin not configured")
	}

	token, err := p.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=%s", token)

	requestData := TemplateMessageRequest{
		ToUser:     p.config.OpenID,
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

	log.Printf("[WeChat Plugin] Message sent successfully, msgid: %d", apiResp.Msgid)
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
