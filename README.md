<p align="center">
    <img height="124px" src="https://github.com/Wuqiyang312/gotify-wechat-plugin/blob/main/img/logo.png" />
</p>

<h1 align="center">Gotify 微信推送插件</h1>

<p align="center">
    通过微信公众号模板消息，将 Gotify 通知实时转发到微信。
</p>

<p align="center">
    <a href="https://github.com/Wuqiyang312/gotify-wechat-plugin/releases/latest"><img src="https://img.shields.io/github/v/release/Wuqiyang312/gotify-wechat-plugin" alt="Release"></a>
    <a href="https://github.com/Wuqiyang312/gotify-wechat-plugin/blob/main/License"><img src="https://img.shields.io/github/license/Wuqiyang312/gotify-wechat-plugin" alt="License"></a>
    <a href="https://github.com/Wuqiyang312/gotify-wechat-plugin/actions"><img src="https://img.shields.io/github/actions/workflow/status/Wuqiyang312/gotify-wechat-plugin/release.yml" alt="Build"></a>
</p>

## 功能特性

- **消息流实时转发** — 通过 WebSocket 监听 Gotify 消息流，自动将匹配的消息转发到微信
- **消息路由** — 按应用 ID 精确匹配或使用 `*` 通配符转发所有消息
- **多接收者** — 支持同时推送给多个微信用户，并发发送
- **Webhook 接口** — 提供 `/send` 和 `/test` HTTP 端点，支持外部系统集成
- **安全的 Token 管理** — access_token 自动缓存，过期前 5 分钟自动刷新，双重检查锁避免并发问题
- **运行状态监控** — 在 Gotify WebUI 中实时查看发送统计、连接状态和错误信息
- **自动重连** — WebSocket 断线后指数退避重连（1s ~ 2min）
- **CI 自动构建** — 跟踪 Gotify Server 上游版本，自动对齐依赖并发布

## 工作原理

```
Gotify Server ──WebSocket /stream──> 插件 (消息路由匹配) ──模板消息 API──> 微信用户
                                       │
                 外部系统 ──POST /send──┘
```

插件启用后，如果配置了 `client_token` 和 `message_routes`，会自动建立 WebSocket 连接监听 Gotify 消息流。收到的消息经路由规则匹配后，通过微信公众号模板消息 API 推送给所有配置的接收者。

同时插件注册了 Webhook 端点，支持外部系统直接调用发送消息。

## 环境要求

- Gotify Server v2.4.0+
- 微信公众号（服务号，需要模板消息权限）
- 微信消息模板需包含字段：`title`、`content`

## 安装

### 方式一：下载预编译文件

从 [Releases](https://github.com/Wuqiyang312/gotify-wechat-plugin/releases/latest) 页面下载对应架构的 `.so` 文件：

| 文件 | 架构 |
|------|------|
| `gotify-wechat-plugin-linux-amd64.so` | x86_64 |
| `gotify-wechat-plugin-linux-arm64.so` | ARM64 (树莓派 4 等) |
| `gotify-wechat-plugin-linux-arm-7.so` | ARMv7 (树莓派 3 等) |

将 `.so` 文件放入 Gotify 插件目录后重启 Gotify：

```bash
cp gotify-wechat-plugin-linux-amd64.so /path/to/gotify/plugins/
systemctl restart gotify
```

### 方式二：手动构建

构建需要 Docker 环境（使用 `gotify/build` 官方镜像确保与 Gotify Server 的 Go 版本一致）：

```bash
# 对齐依赖版本
make check-compat

# 构建指定平台
make build-linux-amd64    # x86_64
make build-linux-arm64    # ARM64
make build-linux-arm-7    # ARMv7

# 或构建所有平台
make all
```

构建产物位于 `build/` 目录。

## 配置

登录 Gotify WebUI → 插件 → 微信推送 → 配置，填写以下参数：

### 基础配置（必填）

| 参数 | 说明 | 示例 |
|------|------|------|
| `appid` | 微信公众号 AppID，以 `wx` 开头 | `wx1234567890abcdef` |
| `app_secret` | 微信公众号 AppSecret | |
| `template_id` | 微信模板消息 ID | |

### 接收者配置（二选一，至少配置一项）

**单接收者模式（向后兼容）：**

| 参数 | 说明 |
|------|------|
| `openid` | 目标用户的 OpenID |

**多接收者模式：**

| 参数 | 说明 |
|------|------|
| `recipients` | 接收者数组，每项包含 `name`（名称，不可重复）和 `openid` |

配置示例：

```json
{
  "recipients": [
    { "name": "张三", "openid": "oXXXX_user1" },
    { "name": "李四", "openid": "oXXXX_user2" }
  ]
}
```

### 消息流配置（可选）

配置后插件会通过 WebSocket 自动监听 Gotify 消息并转发。

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `client_token` | Gotify 客户端 Token（配置 `message_routes` 时必填） | |
| `message_routes` | 消息路由规则数组 | `[]` |
| `gotify_url` | Gotify 服务器地址 | `http://localhost` |

**路由规则说明：**

- 路径末尾的数字会被解析为应用 ID，如 `messages/1` 匹配 appid=1 的消息
- `*` 通配符匹配所有消息

```json
{
  "client_token": "your-gotify-client-token",
  "message_routes": [
    { "path": "messages/1" },
    { "path": "messages/3" }
  ]
}
```

转发所有消息：

```json
{
  "client_token": "your-gotify-client-token",
  "message_routes": [
    { "path": "*" }
  ]
}
```

### 其他配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `jump_url` | 点击微信消息后跳转的链接 | `https://127.0.0.1` |

## 使用方法

### 自动转发（推荐）

配置好消息流后，Gotify 收到的消息会自动匹配路由规则并转发到微信，无需额外操作。

### Webhook 手动发送

```bash
curl -X POST https://your-gotify-server/plugin/{id}/custom/wechat/send \
  -H "Content-Type: application/json" \
  -d '{
    "title": "告警通知",
    "content": "服务器 CPU 使用率超过 90%"
  }'
```

### 测试连接

```bash
curl https://your-gotify-server/plugin/{id}/custom/wechat/test
```

或在 Gotify WebUI 插件显示页面中点击「Send Test Message」链接。

## 微信模板设置

在微信公众平台创建模板，需包含 `title` 和 `content` 两个字段：

```
标题：{{title.DATA}}
内容：{{content.DATA}}
```

## 运行状态监控

插件在 Gotify WebUI 的显示页面中提供以下信息：

- 插件启用/禁用状态
- 配置摘要（敏感信息自动脱敏，仅显示前 4 位和后 4 位）
- 接收者列表
- 消息统计：总发送数、总失败数、最后发送时间
- 消息流连接状态和路由规则
- 最近一次错误信息

插件还会通过 Gotify 消息通知以下事件：

| 事件 | 优先级 |
|------|--------|
| 插件启用/停用 | 2 |
| 消息推送成功 | 1 |
| 消息推送失败 | 5 |

## 项目结构

```
.
├── main.go          # 插件入口，注册 Gotify 插件信息
├── wechat.go        # 核心逻辑：消息发送、Webhook、Token 管理、状态展示
├── config.go        # 配置结构定义与校验
├── stream.go        # WebSocket 消息流监听与路由
├── Makefile         # 构建脚本（Docker 交叉编译）
├── .github/
│   └── workflows/
│       ├── release.yml       # Tag 触发：自动构建并发布 Release
│       └── sync-version.yml  # 每日检查：跟踪 Gotify Server Go 版本变更
└── img/
    └── logo.png
```

## 开发

```bash
make fmt      # 格式化代码
make vet      # 静态检查
make test     # 运行测试
make all      # 构建所有平台
make clean    # 清理构建产物
```

## 常见问题

### 插件加载失败

- 插件必须与 Gotify Server 使用相同版本的 Go 构建，使用 `make check-compat` 对齐依赖
- 直接下载 [Release](https://github.com/Wuqiyang312/gotify-wechat-plugin/releases/latest) 中的预编译文件可避免版本不匹配
- 检查 `.so` 文件权限

### 消息发送失败

- 确认 AppID（以 `wx` 开头）、AppSecret、模板 ID 配置正确
- 确认模板已审核通过且包含 `title`、`content` 字段
- 确认 OpenID 有效（用户已关注公众号）
- 查看 Gotify 日志中 `[WeChat Plugin]` 前缀的日志

### 消息流无法连接

- 确认 `client_token` 是有效的 Gotify 客户端 Token
- 确认 `gotify_url` 可达（默认 `http://localhost`，Docker 部署时可能需要修改）
- 插件会自动重连，可在 WebUI 显示页面查看连接状态

### Token 错误

- access_token 自动缓存并在过期前 5 分钟刷新
- 如持续报错，检查 AppID 和 AppSecret 是否正确
- 检查服务器网络是否能访问 `api.weixin.qq.com`

## 许可证

[MIT License](License)

## 相关链接

- [Gotify](https://gotify.net/)
- [Gotify 插件开发文档](https://gotify.net/docs/plugin)
- [Gotify 插件 API](https://github.com/gotify/plugin-api)
- [微信公众平台](https://mp.weixin.qq.com/)
- [微信模板消息接口文档](https://developers.weixin.qq.com/doc/offiaccount/Message_Management/Template_Message_Interface.html)
