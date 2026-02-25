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

// RouteMatch 路由匹配条件
type RouteMatch struct {
	AppID       []int `yaml:"app_id" json:"app_id"`
	MinPriority *int  `yaml:"min_priority" json:"min_priority"`
}

// Route 消息路由规则
type Route struct {
	Name       string     `yaml:"name" json:"name"`
	Match      RouteMatch `yaml:"match" json:"match"`
	Recipients []string   `yaml:"recipients" json:"recipients"`
}

// Config 插件配置
type Config struct {
	AppID      string `yaml:"appid" json:"appid"`
	AppSecret  string `yaml:"app_secret" json:"app_secret"`
	TemplateID string `yaml:"template_id" json:"template_id"`
	JumpURL    string `yaml:"jump_url" json:"jump_url"`

	// 向后兼容：单 OpenID 模式
	OpenID string `yaml:"openid" json:"openid"`

	// 多接收者 + 路由模式
	Recipients []Recipient `yaml:"recipients" json:"recipients"`
	Routes     []Route     `yaml:"routes" json:"routes"`
}

func (p *WeChatPlugin) DefaultConfig() interface{} {
	return &Config{
		AppID:      "",
		AppSecret:  "",
		OpenID:     "",
		TemplateID: "",
		JumpURL:    "https://push.hzz.cool",
		Recipients: []Recipient{},
		Routes:     []Route{},
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

	// 验证 Routes
	for i, route := range config.Routes {
		if strings.TrimSpace(route.Name) == "" {
			return fmt.Errorf("route[%d]: name is required", i)
		}
		if len(route.Recipients) == 0 {
			return fmt.Errorf("route[%d] %q: at least one recipient is required", i, route.Name)
		}
		for _, rName := range route.Recipients {
			if !recipientNames[rName] {
				return fmt.Errorf("route[%d] %q: unknown recipient %q", i, route.Name, rName)
			}
		}
	}

	if strings.TrimSpace(config.JumpURL) == "" {
		config.JumpURL = "https://push.hzz.cool"
	}

	p.mu.Lock()
	p.config = config
	p.mu.Unlock()

	return nil
}
