# WorkBuddy AI CLIProxyAPI 代理插件方案 (workbuddy-ai-cliproxy)

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://golang.org)
[![CLIProxyAPI Compatibility](https://img.shields.io/badge/CLIProxyAPI-v6%20%7C%20v7-success.svg)](https://github.com/router-for-me/CLIProxyAPI)

> **将腾讯 WorkBuddy / CodeBuddy（国内区 + 国际区）深度集成进 CLIProxyAPI (CPA) 的企业级高可用代理插件方案。**
> 
> 支持将 WorkBuddy 账号池转换为标准 **OpenAI Chat Completions**、**Anthropic Claude Messages** 与 **Google Gemini GenerateContent** 接口，无缝接入 Cursor、VS Code Copilot、Claude Code、Windsurf、Cline、Roo Code 等主流编程客户端。

---

## 目录
- [核心亮点](#核心亮点)
- [系统架构概览](#系统架构概览)
- [支持的模型清单](#支持的模型清单)
- [双区架构与凭证规范](#双区架构与凭证规范)
- [核心机制解析](#核心机制解析)
  - [1. 国际区每日 30 积分打卡与国内区签到](#1-国际区每日-30-积分打卡与国内区签到)
  - [2. 进程内账号智能故障转移 (Failover)](#2-进程内账号智能故障转移-failover)
  - [3. 单账号并发准入控制 (Admission Gate)](#3-单账号并发准入控制-admission-gate)
  - [4. 软会话粘性 (Soft Session Affinity) 与 Prompt Cache 保护](#4-软会话粘性-soft-session-affinity-与-prompt-cache-保护)
  - [5. 动态模型管理器与前缀路由](#5-动态模型管理器与前缀路由)
- [快速开始](#快速开始)
  - [1. 编译动态库](#1-编译动态库)
  - [2. 安装至 CLIProxyAPI](#2-安装至-cliproxyapi)
  - [3. 配置与启动](#3-配置与启动)
  - [4. 客户端对接示例](#4-客户端对接示例)
- [内置 Web 管理面板](#内置-web-管理面板)
- [项目代码结构](#项目代码结构)
- [免责声明与许可](#免责声明与许可)

---

## 核心亮点

1. **双区原生融合 (CN + Global Dual-Realm)**：
   - 自动识别 WorkBuddy 国内区（`copilot.tencent.com` / `workbuddy.cn`）与国际区（`workbuddy.ai` / `codebuddy.ai`），解决海外账号被错误路由至国内网关报 401 网页重定向的顽疾。
   - 提供隔离前缀路由（`cn/` 与 `cb/` 或 `wb/`），避免同名模型冲突。
2. **零人工干预的每日积分白嫖引擎 (Daily Growth Reward)**：
   - **国际区**：通过向 `POST /v2/report` 发送保活活跃事件触发 Growth Centre 热力图，次日自动入账 **30 积分/天（Bonus Pack，30天有效）**，并支持 14 天/28 天连续打卡奖励累加；新号一键激活 250 积分 Expert 权益。
   - **国内区**：自动执行每日签到，稳定入账 **100 积分/天 + 500 积分/月**。
   - 内置防重入、热力图反向探测与 4-Tick 分布式退避调度（08:00 / 12:00 / 16:00 / 20:00）。
3. **插件级零感知故障转移 (In-Plugin Smart Failover)**：
   - 针对 C-Shared 动态库无法复用宿主 Selector 重试机制的问题，插件内建多账号轮询与自动重试链。
   - 遭遇 **429 频率限制**、**402 / 14018 积分耗尽** 或 **6004 网关抖动** 时，无损切流到同 Realm 可用账号，不向客户端吐报错。
4. **防封控并发准入与会话粘性 (Admission & Affinity)**：
   - 严格单账号 **最大 2 并发槽位** 控制，杜绝多 IDE 客户端并发暴刷触发上游封号风险。
   - 基于会话哈希的**软会话粘性**，优先复用历史账号以最大化触发上游 Prompt Cache，降本 90% 以上；账号满载或冷却时平滑转移并不折返。
5. **多协议全双工转换 (Multi-Protocol Translation)**：
   - 完整支持 **OpenAI Chat / Stream**、**Claude Messages / Stream (含 Thinking)**、**Gemini GenerateContent**。
   - 深度支持 Function Calling / Tool Use 映射，支持多模态图片/文件输入。
6. **企业级用量审计与控制台 (Observability & Web UI)**：
   - 仅记录去隐私元数据（Token 消耗、上游 Credit 扣费、耗时、模型、命中状态）至高性能 JSONL，附带分析报告脚本。
   - 嵌入式 Web 控制台支持中英双语、卡片/表格双视图、积分包到期倒计时、模型即时连通性测试。

---

## 系统架构概览

```
+-----------------------------------------------------------------------------------+
|                        Client Applications (Cursor, VSCode, Claude Code)          |
+-----------------------------------------------------------------------------------+
                                          |
                [HTTP / SSE] OpenAI / Claude / Gemini API Protocols
                                          v
+-----------------------------------------------------------------------------------+
|                            CLIProxyAPI Host Core (CPA)                            |
|    - Port Listening (:8317)       - Management API Engine      - Static Asset Proxy|
+-----------------------------------------------------------------------------------+
                                          |
                  Go C-Shared Dynamic ABI (.dll / .so / .dylib)
                                          v
+===================================================================================+
|                        WorkBuddy AI Proxy Plugin Engine                           |
|                                                                                   |
|  +-------------------------+  +--------------------------+  +------------------+  |
|  |     Model Registry      |  |   Multi-Protocol Engine  |  | Admission Gate   |  |
|  | - Dynamic Upstream Sync |  | - OpenAI Chat / Responses|  | - Max 2 Slots/Acc|  |
|  | - Realm Prefix (cn/wb)  |  | - Claude Messages + Think|  | - Fast Fail/Rej  |  |
|  | - Custom Fallbacks      |  | - Gemini GenerateContent |  +------------------+  |
|  +-------------------------+  +--------------------------+  +------------------+  |
|                                                              | Session Affinity |  |
|  +-------------------------------------------------------+  | - Prompt Cache   |  |
|  |             Failover & Resilience Manager             |  | - Digest Binding |  |
|  | - 429 Model Quarantine    - 14018 Credit Outage       |  +------------------+  |
|  | - Upstream 6004 Retry     - Auto Realm Isolation      |                        |
|  +-------------------------------------------------------+                        |
|                                                                                   |
|  +-------------------------+  +--------------------------+  +------------------+  |
|  |   Growth & Bonus Center |  |   Auth Lifecycle Core    |  | Usage Audit Log  |  |
|  | - Global: 30 Credit/Day |  | - Token Auto-Refresh     |  | - JSONL Rotation |  |
|  | - CN: 100 Credit/Day    |  | - Balance / Meter Query  |  | - Credit Extract |  |
|  | - 4-Tick Paced Runner   |  | - Multi-Account Pool     |  | - No-leak Safety |  |
|  +-------------------------+  +--------------------------+  +------------------+  |
|                                                                                   |
|  +-----------------------------------------------------------------------------+  |
|  |           Embedded Web Dashboard (/v0/management/plugins/workbuddy)          |  |
|  +-----------------------------------------------------------------------------+  |
+===================================================================================+
                                 |                |
             [HTTPS / SSE] CN Gateway    [HTTPS / SSE] Global Gateway
                                 v                v
                  +--------------------+    +--------------------+
                  |  copilot.tencent   |    |  workbuddy.ai      |
                  |  workbuddy.cn      |    |  codebuddy.ai      |
                  +--------------------+    +--------------------+
```

---

## 支持的模型清单

插件在运行时会通过 `/v2/enterprises/personal/models` 与 `/v3/config` 动态拉取当前用户权限下的真实可用模型，并支持静态注册兜底：

| 模型标识 (Model ID) | 上游厂商 | 适用场景 | 说明 |
| :--- | :--- | :--- | :--- |
| **gpt-6-astra** / **cb/gpt-6-astra** | OpenAI | 极限复杂编码、超长链路推理 | 国际区独享旗舰模型 |
| **gpt-6.1-sol** / **cb/gpt-6.1-sol** | OpenAI | 高效日常编码、低时延推理 | 性价比最高，支持缓存加速 |
| **gpt-6-luna** / **cb/gpt-6-luna** | OpenAI | 高吞吐快速补全、代码扫描 | 极速流式响应 |
| **claude-fable-5-1** / **cb/claude-fable-5-1** | Anthropic | 极高智商复杂架构重构 | 深度逻辑推理模型 |
| **claude-opus-5-5** / **cb/claude-opus-5-5** | Anthropic | Agentic 全自主智能体开发 | 顶尖工具调用能力 |
| **claude-sonnet-5-5** / **cb/claude-sonnet-5-5** | Anthropic | 综合日常开发首选 | 速度与智能的黄金平衡 |
| **gemini-3.8-flash** / **cb/gemini-3.8-flash** | Google | 超长百万上下文与多模态 | 极低单价，超速输出 |
| **deepseek-v4** / **deepseek-v3** | DeepSeek | 通用代码生成与审查 | 国内区/国际区均稳定支持 |
| **hunyuan-turbo** / **cn/hunyuan-turbo** | 腾讯混元 | 中文环境语义深度理解与合规 | 国内区核心基础模型 |
| **glm-5.3** / **cn/glm-5.3** | 智谱 AI | 中文代码生成与多轮任务 | 国内区高阶 Coding 优选 |

*注：国内区模型默认可通过 `cn/` 前缀显式指定（如 `cn/deepseek-v3`），国际区可通过 `cb/` 或 `wb/` 显式路由，避免两端同名模型冲突。*

---

## 双区架构与凭证规范

凭证以标准 JSON 文件存放在 CLIProxyAPI 的 `auths/` 目录中，命名遵循 `workbuddy-<uid>-<realm>.json`：

```json
{
  "type": "workbuddy",
  "id": "workbuddy-10086-intl",
  "realm": "global",
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "expires_at": 1740000000,
  "user_id": "10086",
  "email": "developer@example.com",
  "nickname": "WorkBuddy-Intl-01",
  "credits": 280.0,
  "disabled": false
}
```

- **国内区凭证 (CN)**：`realm: "cn"`，上游域名 `copilot.tencent.com` / `workbuddy.cn`，请求附带 `X-Domain: copilot.tencent.com`。
- **国际区凭证 (Global)**：`realm: "global"`，上游域名 `workbuddy.ai` / `codebuddy.ai`，请求附带 `X-Domain: codebuddy.ai`。

---

## 核心机制解析

### 1. 国际区每日 30 积分打卡与国内区签到

- **国际区底层原理**：
  上游成长中心在收到客户端发送的 `POST /v2/report` 事件（携带 `eventCode: "chat_request_send"`）后，标记今日活跃。通过 `GET /activity/growth/heatmap` 校验当日单元格 `score >= 2`。次日清晨（中国时间早间），系统自动发放 30 积分的 Bonus Pack（`TCACA_code_007_*`，有效期 30 天）。
- **国内区原理**：
  直接请求 `POST /v2/checkin` 领取每日 100 积分奖励。
- **定时退避保护**：
  内置调度器在本地时间 `08:00 / 12:00 / 16:00 / 20:00` 巡检。如果今日已点亮（Lit），则立即跳过，绝不重复触发，每次上报间隔 1.5 秒，严防突发流量导致 429。

### 2. 进程内账号智能故障转移 (Failover)

CLIProxyAPI 官方动态库加载机制为 C-Shared，上游请求失败由插件自主接管。
- 遇 **HTTP 429 / code 6004**：将当前账号在当前模型上拉入临时冷却队列（默认 30 秒至 5 分钟退避），在同 Realm 寻找备用可用账号立即透明重试；
- 遇 **HTTP 402 / code 14018 (积分耗尽)**：将该账号标记为 Quota Exhausted（自动隔离），切换后续账号；
- **非破坏性**：不修改或注销磁盘上的 auth 文件，重启或额度刷新后自动解除隔离。

### 3. 单账号并发准入控制 (Admission Gate)

- 上游针对高频并发有严格的封号防刷策略。
- 插件在内存中维护每个 `AccountID` 的并发槽位计数器（默认上限为 2）。
- 请求到达时先获取槽位；若已满，立即平滑将请求调度至其它空闲账号，杜绝因并发排队导致的客户端超时。

### 4. 软会话粘性 (Soft Session Affinity) 与 Prompt Cache 保护

- 现代大模型（如 GPT-6.1 Sol / Claude 5.5）具备高达 90% 的上下文缓存优惠。跨账号切换会导致缓存完全击穿。
- 插件提取客户端的对话标识（Session ID / Hash），建立 `Session -> AccountID` 的内存映射（有效期 1 小时）。
- 该账号空闲时持续锁定同账号；若发生并发占满或上游风控，则降级转移至替代账号，并将新会话绑定到替代账号，杜绝频繁往返震荡。

---

## 快速开始

### 1. 编译动态库

确保安装 Go 1.24+ 与 C 编译器（GCC / MinGW-w64）：

```bash
# Windows 环境 (生成 workbuddy.dll)
./build.bat

# Linux / macOS 环境 (生成 workbuddy.so / workbuddy.dylib)
make build
```

编译产物将输出在 `bin/` 目录下。

### 2. 安装至 CLIProxyAPI

将生成的二进制动态库拷贝至 CLIProxyAPI 的 `plugins/` 目录：

```bash
# 复制到 CPA 插件根目录
cp bin/workbuddy.dll /path/to/cliproxyapi/plugins/
# 或平台分级目录
# plugins/windows/amd64/workbuddy.dll
```

### 3. 配置与启动

在 CLIProxyAPI 的 `config.yaml` 中启用插件配置：

```yaml
plugins:
  enabled: true
  configs:
    workbuddy:
      daily_bonus: true      # 开启国际区每日 30 积分自动打卡
      checkin_auto: true     # 开启国内区每日 100 积分自动签到
      max_concurrency: 2     # 单账号最大并发数 (建议 2)
      cn_prefix: "cn/"       # 国内区模型前缀
      global_prefix: "cb/"   # 国际区模型前缀
```

启动 CLIProxyAPI：
```bash
./cli-proxy-api -config config.yaml
```

### 4. 客户端对接示例

启动后，CLIProxyAPI 将在 `http://127.0.0.1:8317` 暴露标准接口：

- **OpenAI 兼容客户端（如 Cursor / NextChat）**：
  - Base URL: `http://127.0.0.1:8317/v1`
  - API Key: 任意字符串（或 CPA 设置的 client-key）
  - Model: `cb/gpt-6.1-sol` 或 `cn/deepseek-v3`
- **Claude Code 终端工具**：
  ```bash
  export ANTHROPIC_BASE_URL="http://127.0.0.1:8317"
  export ANTHROPIC_API_KEY="dummy"
  claude --model cb/claude-sonnet-5-5
  ```

---

## 内置 Web 管理面板

访问 CLIProxyAPI 管理中心，打开插件管理 -> **WorkBuddy AI**：
- **账号看板**：查看所有 CN 与 Global 账号的实时积分、各积分包剩余天数、今日调用量、实时并发状态。
- **快捷动作**：支持一键触发批量打卡/签到、一键激活 250 积分 Expert 权益、一键探测连通性。
- **模型配置**：可视化勾选上架/下架模型，自定义别名与跨模型 Fallback。
- **用量审计**：按日期、账号、模型查询实际 Token 与上游扣除 Credit 趋势。

---

## 项目代码结构

```
workbuddy-ai-cliproxy/
├── cmd/workbuddy-plugin/     # C-Shared 动态库导出入口
├── internal/
│   ├── auth/                 # 凭据存储、Token 刷新与余额解析
│   ├── executor/             # CPA ProviderExecutor 实现与流式处理
│   ├── failover/             # 并发准入、软会话粘性与故障隔离
│   ├── growth/               # 国际区 30 积分打卡与国内区签到引擎
│   ├── models/               # 动态模型清单与前缀路由
│   ├── translator/           # OpenAI/Claude/Gemini 协议互转
│   ├── usage/                # 隐私安全 JSONL 审计日志
│   └── management/           # 管理控制台 REST API
├── web/                      # 嵌入式管理控制台静态资源
├── docs/                     # 协议逆向与架构深度设计白皮书
├── Makefile                  # 多平台编译规则
└── build.bat                 # Windows 构建脚本
```

---

## 许可证
本项目采用 [MIT 许可证](LICENSE)。本方案仅供开发者学习研究大型模型网关架构与协议适配使用，请遵守上游服务商用户协议与服务条款。
