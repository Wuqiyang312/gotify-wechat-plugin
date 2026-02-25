<p align="center">
    <img height="124px" src="https://github.com/Wuqiyang312/gotify-wechat-plugin/blob/main/img/logo.png" />
</p>


# Gotify 微信推送插件

一个将 Gotify 消息转发到微信的插件。

## 功能特性

- 将 Gotify 消息转发到微信
- 安全的 Token 缓存
- 支持模板消息
- 通过 Gotify WebUI 轻松配置
- 提供 Webhook 接口支持外部集成

## 环境要求

- Gotify Server v2.4.0+
- 微信服务号（需要模板消息权限）
- 微信模板需包含字段：`title`、`content`

## 安装

### 1. 构建插件

```bash
# 检查依赖兼容性
make check-compat

# 为目标平台构建
make build-linux-amd64    # x86_64 架构
make build-linux-arm64    # ARM64 架构
make build-linux-arm-7    # ARM v7 架构
```

### 2. 部署到 Gotify

将构建好的 `.so` 文件复制到 Gotify 插件目录：

```bash
cp build/gotify-wechat-plugin-linux-amd64.so /path/to/gotify/plugins/
```

### 3. 重启 Gotify

```bash
systemctl restart gotify
```

## 配置

1. 登录 Gotify WebUI
2. 进入 **插件** 页面
3. 找到 **微信推送** 插件
4. 点击 **配置** 并填写：
   - **AppID**: 微信服务号 AppID
   - **AppSecret**: 微信服务号 AppSecret
   - **OpenID**: 目标用户的 OpenID
   - **模板ID**: 微信模板消息 ID
   - **跳转URL**: （可选）点击消息跳转的链接

5. 点击 **保存** 并 **启用** 插件

## 使用方法

### 通过 Webhook 发送

向 Webhook 端点发送 POST 请求：

```bash
curl -X POST https://your-gotify-server/plugin/1/custom/wechat/send \
  -H "Content-Type: application/json" \
  -d '{
    "title": "告警",
    "content": "服务器 CPU 使用率过高！"
  }'
```

### 测试连接

点击插件显示页面中的"发送测试消息"链接。

## 微信模板设置

你的微信模板应包含以下字段：

```
{{title.DATA}}
{{content.DATA}}
```

示例模板：
```
标题：{{title.DATA}}
内容：{{content.DATA}}
```

## 项目结构

```
.
├── main.go       # 插件入口
├── wechat.go     # 核心插件逻辑
├── config.go     # 配置管理
├── go.mod        # Go 模块定义
├── Makefile      # 构建脚本
└── README.md     # 说明文档
```

## 本地构建

```bash
# 格式化代码
make fmt

# 运行检查
make vet

# 运行测试
make test

# 构建所有平台
make all
```

## 常见问题

### 插件加载失败

- 确保插件与 Gotify 使用相同版本的 Go 构建
- 使用 `make check-compat` 检查依赖兼容性
- 检查 `.so` 文件权限是否正确

### 消息发送失败

- 验证微信配置信息是否正确
- 检查模板 ID 是否存在且已审核通过
- 确保 OpenID 有效
- 查看 Gotify 日志中的错误信息

### Token 错误

- 插件会自动缓存 access_token
- Token 会在过期前 5 分钟自动刷新
- 检查 AppID 和 AppSecret 是否正确

## 许可证

MIT License

## 相关链接

- [Gotify](https://gotify.net/)
- [微信公众平台](https://mp.weixin.qq.com/)
- [Gotify 插件 API 文档](https://github.com/gotify/plugin-api)
