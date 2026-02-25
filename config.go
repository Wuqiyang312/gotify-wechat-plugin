package main

import (
	"fmt"
	"strings"
)

// Recipient 接收者配置
type Recipient struct {
	Name   string `yaml:"name" json:"name"`
	OpenID string `yaml:"openid" json:"openid"`
}

// MessageRoute 消息路由规则
type MessageRoute struct {
	Path string `yaml:"path" json:"path"` // 如 "messages/1", "hi/123", "*"
}

// Config 插件配置
type Config struct {
	AppID      string `yaml:"appid" json:"appid"`
	AppSecret  string `yaml:"app_secret" json:"app_secret"`
	TemplateID string `yaml:"template_id" json:"template_id"`
	JumpURL    string `yaml:"jump_url" json:"jump_url"`

	// 向后兼容：单 OpenID 模式
	OpenID string `yaml:"openid" json:"openid"`

	// 多接收者模式
	Recipients []Recipient `yaml:"recipients" json:"recipients"`

	// Gotify 连接配置（自动发现优先，手动覆盖）
	GotifyURL   string `yaml:"gotify_url" json:"gotify_url"`     // 默认空 = 自动发现 http://localhost
	ClientToken string `yaml:"client_token" json:"client_token"` // Gotify client token

	// 消息路由规则
	MessageRoutes []MessageRoute `yaml:"message_routes" json:"message_routes"`
}

func (p *WeChatPlugin) DefaultConfig() interface{} {
	return &Config{
		AppID:         "",
		AppSecret:     "",
		OpenID:        "",
		TemplateID:    "",
		JumpURL:       "https://127.0.0.1",
		Recipients:    []Recipient{},
		GotifyURL:     "",
		ClientToken:   "",
		MessageRoutes: []MessageRoute{},
	}
}

func (p *WeChatPlugin) ValidateAndSetConfig(c interface{}) error {
	config := c.(*Config)

	if strings.TrimSpace(config.AppID) == "" {
		return fmt.Errorf("AppID is required")
	}
	if strings.TrimSpace(config.AppSecret) == "" {
		return fmt.Errorf("AppSecret is required")
	}
	if strings.TrimSpace(config.TemplateID) == "" {
		return fmt.Errorf("TemplateID is required")
	}

	if !strings.HasPrefix(config.AppID, "wx") {
		return fmt.Errorf("invalid AppID format, should start with 'wx'")
	}

	// 至少需要配置一个 OpenID（单模式）或一个 Recipient（多模式）
	hasLegacyOpenID := strings.TrimSpace(config.OpenID) != ""
	hasRecipients := len(config.Recipients) > 0

	if !hasLegacyOpenID && !hasRecipients {
		return fmt.Errorf("at least one OpenID or Recipient is required")
	}

	// 验证 Recipients
	recipientNames := make(map[string]bool)
	for i, r := range config.Recipients {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("recipient[%d]: name is required", i)
		}
		if strings.TrimSpace(r.OpenID) == "" {
			return fmt.Errorf("recipient[%d] %q: openid is required", i, r.Name)
		}
		if recipientNames[r.Name] {
			return fmt.Errorf("recipient[%d]: duplicate name %q", i, r.Name)
		}
		recipientNames[r.Name] = true
	}

	if strings.TrimSpace(config.JumpURL) == "" {
		config.JumpURL = "https://127.0.0.1"
	}

	// 验证消息路由规则
	for i, route := range config.MessageRoutes {
		if strings.TrimSpace(route.Path) == "" {
			return fmt.Errorf("message_routes[%d]: path is required", i)
		}
	}

	// 如果配置了消息路由，则 ClientToken 必填
	if len(config.MessageRoutes) > 0 && strings.TrimSpace(config.ClientToken) == "" {
		return fmt.Errorf("client_token is required when message_routes are configured")
	}

	p.mu.Lock()
	p.config = config
	p.mu.Unlock()

	return nil
}
