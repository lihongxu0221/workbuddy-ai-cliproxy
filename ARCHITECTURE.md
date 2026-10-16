# WorkBuddy AI CLIProxyAPI 代理插件方案：系统架构与全景技术白皮书

本文档系统性阐述 **WorkBuddy AI CLIProxyAPI 代理插件** 的架构设计、上游协议逆向细节、并发准入控制、软会话粘性算法、故障转移机制以及积分福利打卡引擎。

---

## 1. 方案背景与架构定位

### 1.1 CLIProxyAPI (CPA) 插件体系全版本兼容架构 (v6 / v7 / v8)
CLIProxyAPI 是业界领先的高性能 AI 模型本地聚合网关。CPA 在版本演进中提供了丰富且灵活的插件加载与扩展机制：
- **CPA v7 / v8 官方标准 C ABI 体系**：
  - 核心导出符号：`cliproxy_plugin_init`、`cliproxyPluginCall`、`cliproxyPluginFree`、`cliproxyPluginShutdown`；
  - 采用无语言依赖的二进制 C 函数指针表，通过 JSON RPC 信封完成状态与能力交互；
  - 支持 `schema_version` 动态自协商机制（插件自适应协商 `min(hostVersion, 6)`），平滑兼容 CPA v7.1.x ~ v7.3.x 及 v8.0.x，杜绝版本报错；
  - 支持多账号凭据展开（`auth.parse` 返回 `Auths []AuthData` 切片），单个凭据文件可自动展开为多个虚拟 Provider Auth；
  - 支持真流式（Async Streaming）：通过 `host.stream.emit` 与 `host.stream.close` 实时逐字推流，实现与上游零缓冲同步；
  - 完整声明并实现 `ModelProvider`、`AuthProvider`、`Executor`、`ManagementAPI` 与 `QuotaProvider`。
- **CPA v6 及自定义加载器向下兼容体系**：
  - 导出传统符号：`InitPlugin(ctx unsafe.Pointer) unsafe.Pointer`、`RegisterRoutes(routerGroup unsafe.Pointer)`、`UnloadPlugin()`；
  - 保证在早期定制宿主或第三方集成加载器中无缝挂载。
- **独立服务 (Standalone CLI) 模式**：
  - 二进制可作为独立进程运行（`-standalone -port 8318`），提供标准 OpenAI 兼容的 `/v1/chat/completions` 与 `/v1/models` 端点以及 `/ui` 控制台，完美支持不支持 CGO 动态库的 CPA v6 外部网关反代接入。

### 1.2 腾讯 WorkBuddy / CodeBuddy 生态特征
腾讯 WorkBuddy（原 CodeBuddy）是腾讯推出的全场景智能编码与办公智能体工作台：
1. **国内区 (CN Realm)**：
   - 入口：`copilot.tencent.com` / `workbuddy.cn`
   - 鉴权：腾讯云 CAM / 微信扫码，颁发国内区 JWT Token；
   - 积分体系：每日签到（+100 积分/天，每月累计额外奖励 +500 积分）；
   - 模型侧重：腾讯混元（Hunyuan-Turbo/Pro）、智谱 GLM-5.3、DeepSeek-V3/R1。
2. **国际区 (Global / Intl Realm)**：
   - 入口：`workbuddy.ai` / `codebuddy.ai`
   - 鉴权：Google / GitHub / 邮箱登录；
   - 积分体系：月度赠送 100 积分，新账号 250 积分 Expert 试用权益（14天）；**核心每日奖励：成长中心 (Growth Centre) 活跃打卡次日赠送 30 积分/天（Bonus Pack，30天有效期），连续 14 天/28 天打卡阶梯奖励额外加成**；
   - 模型侧重：国际前沿旗舰模型（GPT-6 Astra、GPT-6.1 Sol、Claude Fable 5.1、Claude Opus 5.5、Gemini 3.8 Flash）。

### 1.3 核心痛点与设计目标
- **痛点一：双区域名与网关混淆**。若将国际区凭据发送到国内区网关，上游返回非 JSON 的 401 HTML 登录重定向页面，导致客户端直接解析崩溃。
- **痛点二：C-Shared 插件跨账号重试缺失**。宿主 CPA 的重试选择器针对内置 Provider 生效，但插件如果直接返回 429 或 14018 错误，宿主不会触发账号重选，导致客户端请求中断。
- **痛点三：高频并发封号风控**。上游对单一账号瞬时并发超过 2 次极其敏感，极易触发软封控（429 降智或 Code 6004）。
- **痛点四：Prompt Cache 击穿**。GPT-6 与 Claude 模型在上下文命中 Prompt Cache 时费用直降 90%。若无序轮询账号，会导致每个账号的 Prompt Cache 频繁失效，成本飙升。
- **设计目标**：建立具备**双区智能隔离、多账号并发准入（Max 2）、软会话粘性、插件内无损故障转移、零人工干预每日打卡白嫖**的企业级插件。

---

## 2. 核心架构与模块交互

### 2.1 整体架构分层图

```
                       [ IDE & Coding Tools ]
              (Cursor, VS Code, Claude Code, Windsurf)
                                 │
           HTTP Request (/v1/chat/completions, /v1/messages)
                                 ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    CLIProxyAPI Host Core Framework                      │
│   ├── Authentication Middleware (Downstream API Keys)                   │
│   ├── Gin HTTP Router (:8317)                                           │
│   └── Model Registry Core                                               │
└─────────────────────────────────────────────────────────────────────────┘
                                 │
                     C-Shared Plugin Interface (ABI)
                                 ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                   WorkBuddy AI Dynamic Proxy Plugin                     │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │ 1. Routing & Protocol Translators                                 │  │
│  │    - Realm Prefix Handler (Strip cn/ or cb/ -> Internal Model ID) │  │
│  │    - OpenAI Chat / Anthropic Messages / Gemini Translators        │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                 │                                       │
│  ┌──────────────────────────────▼────────────────────────────────────┐  │
│  │ 2. Failover & Concurrency Engine                                  │  │
│  │    - Soft Session Affinity: Session Hash -> Stable Account        │  │
│  │    - Admission Gate: Max 2 In-flight Request Slots per Account    │  │
│  │    - Circuit Breaker: 429/14018 Account & Model Level Quarantine  │  │
│  │    - In-Plugin Retry Loop: Fallback to Next Eligible Account      │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                 │                                       │
│  ┌──────────────────────────────▼────────────────────────────────────┐  │
│  │ 3. WorkBuddy Upstream Client                                      │  │
│  │    - Fingerprint & Header Normalization (UA, X-Domain, X-User-Id) │  │
│  │    - SSE Stream Decoder & Credit Usage Terminal Extractor         │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                 │                                       │
│  ┌──────────────────────────────▼────────────────────────────────────┐  │
│  │ 4. Background Growth & Maintenance Daemons                        │  │
│  │    - 4-Tick Scheduled Runner (08:00, 12:00, 16:00, 20:00)         │  │
│  │    - Global Realm: POST /v2/report (chat_request_send)            │  │
│  │    - CN Realm: POST /v2/checkin                                   │  │
│  │    - Token Auto-Refresher & Balance Sync Engine                   │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                 │                                       │
│  ┌──────────────────────────────▼────────────────────────────────────┐  │
│  │ 5. Observability & Management Web Console                         │  │
│  │    - JSONL Pure-Metadata Usage Logger (Privacy-Safe)              │  │
│  │    - Embedded Vue/HTML5 Dashboard (/v0/management/plugins/work... │  │
│  └───────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                 │
                 HTTPS / SSE Network Dispatch
                 ┌───────────────┴───────────────┐
                 ▼                               ▼
       [ CN Upstream ]                  [ Global Upstream ]
    copilot.tencent.com                    workbuddy.ai
       workbuddy.cn                        codebuddy.ai
```

---

## 3. 上游协议逆向与报文规范

### 3.1 域名与网关拓扑
| 域 (Realm) | 主入口与 API 网关 | 认证域名 | 报文特征头 |
| :--- | :--- | :--- | :--- |
| **CN (国内区)** | `copilot.tencent.com`<br>`api.workbuddy.cn` | `copilot.tencent.com` | `X-Domain: copilot.tencent.com`<br>`X-User-Id: <uid>` |
| **Global (国际区)** | `workbuddy.ai`<br>`codebuddy.ai` | `www.codebuddy.ai` | `X-Domain: codebuddy.ai`<br>`X-User-Id: <uid>` |

### 3.2 客户端通信身份伪装 (Fingerprint)
为防止被上游识别为第三方恶意爬虫，请求头必须严格保持与官方桌面客户端 Electron 架构一致：
```http
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) WorkBuddy/1.2.8 Chrome/128.0.6613.186 Electron/32.2.1 Safari/537.36
Content-Type: application/json
Authorization: Bearer <access_token>
X-Domain: codebuddy.ai
X-User-Id: 10086
Accept: text/event-stream, application/json
```

### 3.3 对话流式接口与 Credit 结算报文
上游对话请求接口为 `POST /v2/chat/completions` 或 `POST /v2/conversation/stream`。
#### 请求体规范：
```json
{
  "model": "gpt-6.1-sol",
  "messages": [
    {"role": "system", "content": "You are a helpful coding assistant."},
    {"role": "user", "content": "Hello, write a quicksort in Go."}
  ],
  "stream": true,
  "temperature": 0.2
}
```

#### 响应 SSE 报文：
增量数据块以标准 `data: {...}` 传输，末尾数据包携带真实的 Credit 计费消耗与 Token 统计：
```text
data: {"id":"chatcmpl-87a","object":"chat.completion.chunk","choices":[{"delta":{"content":"package"}}]}

data: {"id":"chatcmpl-87a","object":"chat.completion.chunk","choices":[{"delta":{"content":" main"}}]}

data: {"id":"chatcmpl-87a","choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":120,"completion_tokens":45,"total_tokens":165,"prompt_tokens_details":{"cached_tokens":100}},"credit":0.045}

data: [DONE]
```
插件的 `credit_parser` 模块在流结束前拦截并提取 `credit` 字段和 `cached_tokens`，写入本地审计流水账，同时对外按标准 OpenAI 格式抹平。

---

## 4. 国际区 30 积分打卡与成长中心机制

### 4.1 触发链条逻辑
国际区成长中心（Growth Centre）基于用户活跃度驱动奖励发放：
1. **活跃事件定义**：上报一次 `eventCode: "chat_request_send"` 事件至 `POST /v2/report`；
2. **热力图状态变更**：上报成功后，系统在 `GET /activity/growth/heatmap` 的当日格子记录 `score >= 2`；
3. **奖励入账机制**：次日早晨（约中国时间 04:00~06:00），系统自动向该账号下发 **30 积分的 Bonus Pack**（PackageCode 格式为 `TCACA_code_007_*`），有效期为 30 天；
4. **阶梯累计加成**：连续打卡 14 天额外赠送 50 积分，连续打卡 28 天额外赠送 150 积分。

### 4.2 逆向打卡报文
```http
POST https://www.codebuddy.ai/v2/report HTTP/1.1
Host: www.codebuddy.ai
Authorization: Bearer <access_token>
X-Domain: codebuddy.ai
X-User-Id: <user_id>
Content-Type: application/json

[
  {
    "eventCode": "chat_request_send",
    "timestamp": 1740000000000,
    "mode": "craft",
    "conversationId": "wb-session-auto",
    "requestId": "",
    "inputLength": 16,
    "requestModelId": "deepseek-v4-flash",
    "requestModelName": "DeepSeek V4 Flash",
    "presentAt": 1740000000000,
    "rootRequestId": "",
    "parentConversationId": "wb-session-auto",
    "agentName": "default",
    "agentType": "conversation",
    "userId": "<user_id>"
  }
]
```

### 4.3 调度器安全防护设计
- **前置热力图自检**：先执行 `GET /activity/growth/heatmap`。如果今日格子的 score 已大于 0，说明已点亮，直接跳过；
- **单次上报不重试**：上报即使偶发网络波动绝不重试，避免短时间内多次发送被判定为异常行为；
- **4-Tick 巡检容灾**：每天分别在 `08:00 / 12:00 / 16:00 / 20:00` 运行。即使早晨因网络未唤醒失败，中午或下午的 Tick 也会自然补全；
- **批量打卡起搏间隔**：多账号批量打卡时，账号之间强制睡眠 1.5 秒，严禁瞬时并发撞上上游 429。

---

## 5. 高可用弹性调度子系统

### 5.1 单账号并发准入控制 (Admission Gate)
为避免多 IDE 或插件多并发打崩单个 WorkBuddy 账号：
- 内存维护 `sync.Map` 映射 `AccountID -> chan struct{}`（带容量为 2 的缓冲信道）；
- 请求进入时通过 `select { case ch <- struct{}{}: default: ... }` 尝试占槽；
- 若无法获取槽位，判定该账号当前繁忙，立即寻找同 Realm 的下一个空闲可用账号；若所有账号均满载，立即以 503/429 快速失败，绝不盲目排队引发客户端超时断流。

### 5.2 软会话粘性 (Soft Session Affinity)
- **目标**：保证同一段长对话持续请求在同一账号上，充分利用服务端的 KV Cache（提示词缓存）；
- **绑定算法**：
  提取客户端请求中的 `ConversationID`（或对前 N 轮对话文本计算 SHA256 哈希），作为 `SessionKey`；
  在内存绑定表记录 `SessionKey -> AccountID`（TTL 60 分钟）；
- **平滑降级（Soft）**：
  若被绑定的账号遇到 429 冷却或并发满载，则平滑将请求重新调度至其他空闲账号，并将该会话重新绑定至新账号，不会来回跳动。

### 5.3 故障隔离与退避模型 (Quarantine & Circuit Breaker)
| 异常类型 | 触发条件 | 隔离策略 | 恢复条件 |
| :--- | :--- | :--- | :--- |
| **Model 429** | HTTP 429 / Code 6004 | 隔离 `(Account, Model)` 维度 30s~300s 指数退避 | 冷却计时器归零，允许单次探测请求通过 |
| **Credit 耗尽** | HTTP 402 / Code 14018 | 隔离 `Account` 账号全局维度 30 分钟 | 余额刷新检测到新充值或次日重置 |
| **Token 过期** | HTTP 401 / JWT Expired | 触发异步 `Refresh()` 流程，立即换号重试 | Token 刷新成功后自动解除隔离 |

---

## 6. 协议转换与模型规范

### 6.1 前缀隔离与路由规则
通过前缀机制彻底解决同名模型冲突问题：
- `cn/` 前缀：强制路由至国内区账号（例如 `cn/glm-5.3`）；
- `cb/` 或 `wb/` 前缀：强制路由至国际区账号（例如 `cb/gpt-6.1-sol`）；
- 无前缀模型：按配置偏好路由或优先路由到拥有该模型的可用账号池。

### 6.2 协议适配覆盖矩阵
- **OpenAI 兼容层**：`/v1/chat/completions`、`/v1/models`；
- **Anthropic 兼容层**：`/v1/messages`（支持 Claude 思考块 `thinking` 协议与流式还原）；
- **Gemini 兼容层**：`/v1beta/models/{model}:generateContent`、`:streamGenerateContent`。

---

## 7. 安全与合规性设计

1. **零密钥泄露原则**：所有鉴权 Bearer Token 仅保存在本地 `auths/` 目录中，在 Web 控制台与日志中强制进行脱敏掩码展示；
2. **去隐私审计日志**：JSONL 审计日志仅保留时间戳、账号 ID、模型名称、Token 计数和 Credit 消耗，绝不记录任何 Prompt 输入与输出代码明文；
3. **运行时只读约束**：插件绝不越权修改系统环境或其它插件的认证数据，遵守最小权限原则。
