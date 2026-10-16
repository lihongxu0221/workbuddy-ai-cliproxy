# 腾讯 WorkBuddy / CodeBuddy 官方协议逆向规范 (PROTOCOL_SPEC)

本文档整理自腾讯 WorkBuddy 官方桌面端（Electron）及 Web 端的抓包分析与接口逆向成果，供本代理插件开发与协议转换对照使用。

---

## 1. 域名与基础端点

### 1.1 国内区 (CN Realm)
- 主机名：`copilot.tencent.com` / `api.workbuddy.cn` / `workbuddy.cn`
- 协议：`HTTPS`
- 常用端口：`443`
- 网关特征标头：`X-Domain: copilot.tencent.com`

### 1.2 国际区 (Global Realm)
- 主机名：`www.codebuddy.ai` / `workbuddy.ai` / `api.workbuddy.ai`
- 协议：`HTTPS`
- 常用端口：`443`
- 网关特征标头：`X-Domain: codebuddy.ai`

---

## 2. 身份认证与凭证刷新

### 2.1 请求头标准模板 (Request Headers)
每个发往 WorkBuddy 上游的 HTTP 请求均须带上如下标准头以规避反爬检测：

```http
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) WorkBuddy/1.2.8 Chrome/128.0.6613.186 Electron/32.2.1 Safari/537.36
Authorization: Bearer <access_token>
X-Domain: codebuddy.ai
X-User-Id: <user_id>
Content-Type: application/json
Accept: text/event-stream, application/json
```

### 2.2 Token 刷新接口
- **方法**：`POST`
- **路径**：`/v2/auth/refresh` 或 `/v1/oauth/token`
- **请求体**：
```json
{
  "refresh_token": "eyJhbGciOi...",
  "grant_type": "refresh_token"
}
```
- **响应体**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 86400
  }
}
```

---

## 3. 积分资产与资源查询接口

### 3.1 获取用户资源与积分包列表
- **方法**：`POST`
- **路径**：`/v2/billing/meter/get-user-resource`
- **请求体**：`{}`
- **响应体示例**：
```json
{
  "code": 0,
  "data": {
    "totalCredits": 280.5,
    "packages": [
      {
        "packageCode": "TCACA_code_007_nzdH5h4Nl0",
        "packageName": "平台奖励积分 (Bonus Pack)",
        "remainingCredits": 30.0,
        "initialCredits": 30.0,
        "effectiveTime": 1740000000000,
        "expireTime": 1742592000000,
        "status": 1
      },
      {
        "packageCode": "TCACA_free_monthly_001",
        "packageName": "月度免费配额",
        "remainingCredits": 50.5,
        "initialCredits": 100.0,
        "effectiveTime": 1739000000000,
        "expireTime": 1741600000000,
        "status": 1
      }
    ]
  }
}
```

---

## 4. 国际区成长中心打卡与热力图接口

### 4.1 活跃事件打卡上报
- **方法**：`POST`
- **路径**：`/v2/report`
- **请求体**：
```json
[
  {
    "eventCode": "chat_request_send",
    "timestamp": 1740000000000,
    "mode": "craft",
    "conversationId": "wb-auto-report",
    "requestId": "",
    "inputLength": 16,
    "requestModelId": "deepseek-v4-flash",
    "requestModelName": "DeepSeek V4 Flash",
    "presentAt": 1740000000000,
    "rootRequestId": "",
    "parentConversationId": "wb-auto-report",
    "agentName": "default",
    "agentType": "conversation",
    "userId": "10086"
  }
]
```
- **响应体**：
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "success": true
  }
}
```

### 4.2 查询打卡热力图
- **方法**：`GET`
- **路径**：`/activity/growth/heatmap`
- **响应体示例**：
```json
{
  "code": 0,
  "data": {
    "records": [
      {"date": "2026-10-14", "score": 2},
      {"date": "2026-10-15", "score": 2}
    ],
    "streakDays": 5
  }
}
```
*注：`score >= 2` 表示当日打卡已点亮（Lit），次日凌晨会自动派发 30 积分奖励。*

---

## 5. 国内区每日签到接口

- **方法**：`POST`
- **路径**：`/v2/checkin`
- **域名**：`copilot.tencent.com`
- **请求体**：`{}`
- **响应体示例**：
```json
{
  "code": 0,
  "data": {
    "checkinSuccess": true,
    "rewardCredits": 100,
    "monthlyBonusEarned": false
  }
}
```

---

## 6. 模型列表动态获取接口

- **方法**：`GET`
- **路径**：`/v2/enterprises/personal/models` 与 `/v3/config`
- **响应体示例**：
```json
{
  "code": 0,
  "data": {
    "models": [
      {
        "id": "gpt-6-astra",
        "name": "GPT-6 Astra",
        "vendor": "OpenAI",
        "description": "Flagship reasoning and coding model",
        "creditMultiplier": 1.5,
        "contextLength": 200000,
        "isFree": false
      },
      {
        "id": "gpt-6.1-sol",
        "name": "GPT-6.1 Sol",
        "vendor": "OpenAI",
        "creditMultiplier": 1.0,
        "contextLength": 128000,
        "isFree": false
      },
      {
        "id": "claude-sonnet-5-5",
        "name": "Claude Sonnet 5.5",
        "vendor": "Anthropic",
        "creditMultiplier": 1.2,
        "contextLength": 200000,
        "isFree": false
      },
      {
        "id": "deepseek-v4-flash",
        "name": "DeepSeek V4 Flash",
        "vendor": "DeepSeek",
        "creditMultiplier": 0.0,
        "isFree": true
      }
    ]
  }
}
```

---

## 7. 对话与流式响应 (Chat SSE)

- **方法**：`POST`
- **路径**：`/v2/chat/completions`
- **请求体**：
```json
{
  "model": "gpt-6.1-sol",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": true
}
```
- **SSE 流式传输规范**：
```text
data: {"id":"chatcmpl-01","choices":[{"delta":{"content":"Hi"}}]}

data: {"id":"chatcmpl-01","choices":[{"delta":{"content":" there!"}}]}

data: {"id":"chatcmpl-01","choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16},"credit":0.012}

data: [DONE]
```

---

## 8. 错误状态码对照表

| HTTP 状态码 | 业务 Code | 说明 | 插件应对处理策略 |
| :--- | :--- | :--- | :--- |
| **401** | `40101` / `401` | Token 无效或过期 | 触发后台 Refresh，换号立即无损重试 |
| **402** | `14018` | `Credits exhausted` (额度耗尽) | 立即隔离该账号 30 分钟，自动切换同 Realm 备用账号 |
| **429** | `429` | 请求过频 / 速率限制 | 隔离 (Account, Model) 维度，开启指数退避，无损重试 |
| **500 / 502** | `6004` | 上游微服务异常 / 网关抖动 | 换号重试 1 次；若依然失败则降级报错 |
| **403** | `40301` | 账号被封禁或无权访问该模型 | 标记账号异常，剔除出调度池 |
