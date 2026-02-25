package main

import (
	"github.com/gotify/plugin-api"
)

func GetGotifyPluginInfo() plugin.Info {
	return plugin.Info{
		ModulePath:  "github.com/Wuqiyang312/gotify-wechat-plugin",
		Version:     "0.1.1-beta",
		Author:      "Wuqiyang312",
		Website:     "https://github.com/Wuqiyang312/gotify-wechat-plugin",
		Description: "将 Gotify 消息转发到微信",
		License:     "MIT",
		Name:        "微信推送",
	}
}

func NewGotifyPluginInstance(ctx plugin.UserContext) plugin.Plugin {
	return &WeChatPlugin{
		userCtx:    ctx,
		enabled:    false,
		msgHandler: nil,
		storage:    nil,
		config:     nil,
	}
}

func main() {
	panic("this should be built as go plugin")
}
