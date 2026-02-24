package main

import (
	"fmt"
	"strings"
)

type Config struct {
	AppID      string `yaml:"appid" json:"appid"`
	AppSecret  string `yaml:"app_secret" json:"app_secret"`
	OpenID     string `yaml:"openid" json:"openid"`
	TemplateID string `yaml:"template_id" json:"template_id"`
	JumpURL    string `yaml:"jump_url" json:"jump_url"`
}

func (p *WeChatPlugin) DefaultConfig() interface{} {
	return &Config{
		AppID:      "",
		AppSecret:  "",
		OpenID:     "",
		TemplateID: "",
		JumpURL:    "https://push.hzz.cool",
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
	if strings.TrimSpace(config.OpenID) == "" {
		return fmt.Errorf("OpenID is required")
	}
	if strings.TrimSpace(config.TemplateID) == "" {
		return fmt.Errorf("TemplateID is required")
	}

	if !strings.HasPrefix(config.AppID, "wx") {
		return fmt.Errorf("invalid AppID format, should start with 'wx'")
	}

	if strings.TrimSpace(config.JumpURL) == "" {
		config.JumpURL = "https://push.hzz.cool"
	}

	p.mu.Lock()
	p.config = config
	p.mu.Unlock()

	return nil
}
