# WorkBuddy AI 插件故障排查与风控规避手册 (TROUBLESHOOTING)

本文档总结使用本插件过程中的常见错误、原因诊断以及官方风控应对最佳实践。

---

## 1. 常见错误速查与解决方案

### 1.1 错误信息：`parse failed: invalid character '<'`
- **根本原因**：国际区（Global）账号凭据被错误发送到了国内区（CN）网关。国内区网关无法识别海外 Token，返回了 HTML 格式的 401 网页重定向页面（以 `<html...` 开头），导致 JSON 解析器在读取第一个字符 `<` 时崩溃。
- **解决方案**：
  1. 检查 `auths/workbuddy-*.json` 中的 `realm` 字段，确保国际区账号设置了 `"realm": "global"`；
  2. 客户端调用模型时建议带上 `cb/`、`en/` 或 `wb/` 前缀（如 `cb/gpt-6.1-sol`），显式约束路由；
  3. 确认插件版本已启用双区自动识别功能。

### 1.2 错误信息：`Moved Permanently.` (控制台页面空白或仅显示一行超链接)
- **根本原因**：早期版本挂载资源路由时，Gin 引擎对不带斜杠的静态资源路径（`/v0/resource/plugins/workbuddy/ui`）触发了 301 目录重定向；在 CPAMP 的 iframe 中加载 301 时直接渲染了原生的跳转文本，且后续引用的相对路径 CSS/JS 资源因 CPA 精确路由匹配而全部返回 404。
- **解决方案**：
  1. 升级插件至最新版本：插件现已通过 `RenderFullHTML()` 将页面编译为零外部依赖的 Single-File HTML，并在资源路由命中时直接响应 `200 OK`，彻底杜绝 301 重定向与资源丢失；
  2. 重新编译生成 `workbuddy.dll` 并覆盖到 CPA 的 `plugins/` 目录后重启 CPA。

### 1.3 错误信息：`获取授权失败: invalid admin key`
- **根本原因**：CPAMP 网关内置了保留路径校验表（包含 `"plugins"`）。当控制台请求 `/v0/management/plugins/...` 时，CPAMP 强行校验请求是否携带 CPAMP 管理员私钥（Admin Key），导致运行在无状态 iframe 中的前端请求被拦截报 401。
- **解决方案**：
  1. 插件已将前端 API 请求基地址规范化为 `/v0/management/workbuddyai`（路径第一段为插件 ID，不含 `plugins/`），直接命中 CPAMP 的插件免密代理管道；
  2. 重新编译并覆盖插件动态库即可解决。

### 1.4 CPAMP「OAuth 登录」中找不到 WorkBuddy AI
- **根本原因**：CPA 宿主依据插件返回的 `auth.identifier` 在管理接口中注册 OAuth 供应方。此前插件返回旧标识 `"workbuddy"`，导致 CPAMP 无法将其识别为 `"workbuddyai"`。
- **解决方案**：
  1. 插件现已将对外主 AuthProvider Identifier 升级为 `"workbuddyai"`，重启 CPA 后打开 CPAMP「OAuth 登录」面板即可看到 **WorkBuddy AI** 登录卡片；
  2. 亦可直接在左侧点击 **「WorkBuddy 控制台」**，在页面右上角点击 **「⚡ 扫码 / OAuth 登录」** 按钮发起双区登录。

### 1.5 配置文件 `config.yaml` 找不到或 Key 不知道填什么
- **根本原因**：误以为插件需要新建一套独立的配置文件。
- **正确理解**：
  1. 配置文件就是 **CPA（CLIProxyAPI）宿主程序主目录下的 `config.yaml`**（与 `cli-proxy-api.exe` 处于同一目录）；
  2. `remote-management.secret-key` 是 **CPA 宿主自身的管理密码**。在 CPAMP 中连接 CPA 时填写的 **Management Key** 就是它；
  3. `api-keys` 是供 Cursor、Claude Code 等客户端调用的 Bearer Key；
  4. 插件本身的配置直接嵌套在同一个 `config.yaml` 的 `plugins.configs.workbuddyai` 节点下，插件自身不需要也不强制要求单独设置私有 Key。

### 1.6 错误状态码：`HTTP 429 Too Many Requests`
- **根本原因**：
  1. 单个账号短时间内并发发起超过 2 个会话；
  2. 触发了上游每分钟请求数 (RPM) 或单日 Token 吞吐量限制；
  3. 国际区成长中心打卡上报频率过高。
- **解决方案**：
  1. 保持 `max_concurrency: 2`（切勿在配置中将其调大至 5 或 10）；
  2. 多配置 2~3 个账号组成轮转池，插件的 `Failover` 机制会在单个账号 429 时自动无损切流到同 Realm 的健康账号；
  3. 检查调度器设置，确保 `daily_bonus` 每次打卡间隔至少 1.5 秒且先查热力图自检。

### 1.7 错误状态码：`HTTP 402 Payment Required` (Code: 14018, `Credits exhausted`)
- **根本原因**：当前账号所有免费积分及购买积分均已耗尽。
- **解决方案**：
  1. 插件会自动触发隔离（Quarantine），在内存中将该账号临时休眠 30 分钟，并自动尝试下一个同 Realm 账号；
  2. 如果所有账号均耗尽，请进入 WorkBuddy 官网充值，或等待次日每日 30 积分/100 积分到账；
  3. 新账号可点击控制台的 **激活 Expert 权益** 按钮，立即领取 250 积分试用额度。

### 1.8 错误信息：`code: 6004, message: upstream gateway error`
- **根本原因**：上游网关瞬时抖动、模型服务短暂负载过高。
- **解决方案**：插件已在内部集成了退避重试，通常单次重试即可自愈。

---

## 2. 账号安全与风控规避最佳实践

1. **严格遵守并发准入**：永远不要试图通过魔改并发阈值去进行多并发压力测试，桌面端接口以单账号 <= 2 并发为安全水位线；
2. **保护 Prompt Cache 粘性**：开启 `session_affinity: true`，尽量让同一个会话停留在同一个账号上，既节约积分又模拟真实人类开发者的使用习惯；
3. **多账号 IP 隔离（进阶）**：如果有 5 个以上账号，建议在凭证 JSON 中通过 `"proxy_url": "http://user:pass@proxy-ip:port"` 为不同账号绑定独立的住宅或优质机房代理出口；
4. **打卡时间分散化**：内置的 4-Tick（08:00/12:00/16:00/20:00）带随机 Jitter 偏移，避免在整点 00 分 00 秒固定突发上报。
