# WorkBuddy AI CLIProxyAPI 插件部署与实战集成指南

本文档提供完整的插件编译构建、安装部署、CLIProxyAPI 配置及各主流客户端（Cursor、Claude Code、VS Code 等）的对接指引。

---

## 1. 编译前置准备

### 1.1 基础依赖环境
- **Go 语言环境**：Go 1.24 或更高版本；
- **C 编译器**（构建 C-Shared 动态库必需）：
  - **Windows**：MinGW-w64（推荐使用 MSYS2 安装 `gcc` 或 `mingw-w64-x86_64-gcc`）；
  - **Linux**：`gcc` (`build-essential` on Ubuntu/Debian, `gcc` on RHEL/CentOS)；
  - **macOS**：Xcode Command Line Tools (`xcode-select --install`).

### 1.2 确认 CGO 状态
```bash
go env CGO_ENABLED
# 必须输出为 1
```

---

## 2. 编译构建

项目根目录下提供了多平台 `Makefile` 以及 Windows 本地适用的 `build.bat`。

### 2.1 Windows 平台构建
在终端中直接运行：
```cmd
build.bat
```
或者手动执行：
```bash
go build -buildmode=c-shared -ldflags="-s -w" -o bin/workbuddy.dll ./cmd/workbuddy-plugin
```

### 2.2 Linux 平台构建 (amd64 / arm64)
```bash
make build-linux
```

### 2.3 macOS 平台构建 (darwin/arm64)
```bash
make build-darwin
```

---

## 3. 安装与配置 CLIProxyAPI

### 3.1 放置动态库文件
将编译生成的二进制动态库文件拷贝至 CLIProxyAPI 宿主程序的 `plugins/` 目录中：
```text
cliproxyapi/
├── cli-proxy-api.exe (或 cli-proxy-api)
├── config.yaml
├── auths/
└── plugins/
    └── workbuddy.dll  (Linux 为 workbuddy.so，macOS 为 workbuddy.dylib)
```

### 3.2 配置 `config.yaml`
在 CLIProxyAPI 的配置文件 `config.yaml` 中增加或完善如下节点：

```yaml
port: 8317
auth-dir: "auths"

# 启用插件支持
plugins:
  enabled: true
  configs:
    workbuddy:
      daily_bonus: true      # 国际区每日 30 积分自动打卡
      checkin_auto: true     # 国内区每日 100 积分自动签到
      max_concurrency: 2     # 单账号最大活跃并发数 (建议 2)
      cn_prefix: "cn/"       # 前缀路由定义
      global_prefix: "cb/"
      session_affinity: true # 启用软会话粘性以保护 Prompt Cache

remote-management:
  allow-remote: true
  secret-key: "YourSecretAdminKeyHere"
```

---

## 4. 添加 WorkBuddy 认证凭据

在 CLIProxyAPI 的 `auths/` 目录中创建凭据文件，格式统一为 `workbuddy-<账号标识>-<realm>.json`。

### 4.1 国际区账号样例 (`auths/workbuddy-01-intl.json`)
```json
{
  "type": "workbuddy",
  "id": "workbuddy-intl-01",
  "realm": "global",
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "expires_at": 1740000000,
  "user_id": "1008611",
  "email": "developer-intl@example.com",
  "nickname": "WorkBuddy-Intl-Main",
  "credits": 280.0,
  "disabled": false
}
```

### 4.2 国内区账号样例 (`auths/workbuddy-02-cn.json`)
```json
{
  "type": "workbuddy",
  "id": "workbuddy-cn-01",
  "realm": "cn",
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "expires_at": 1740000000,
  "user_id": "2009822",
  "email": "developer-cn@qq.com",
  "nickname": "WorkBuddy-CN-Main",
  "credits": 650.0,
  "disabled": false
}
```

---

## 5. 启动与验证

### 5.1 启动 CLIProxyAPI 服务
```bash
./cli-proxy-api -config config.yaml
```

### 5.2 验证模型列表与接口连通性
- **查询已加载模型**：
```bash
curl -s http://127.0.0.1:8317/v1/models
```

- **测试对话接口 (OpenAI 协议)**：
```bash
curl -X POST http://127.0.0.1:8317/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "cb/gpt-6.1-sol",
    "messages": [{"role": "user", "content": "Ping!"}],
    "stream": false
  }'
```

---

## 6. 主流客户端接入实战

### 6.1 Cursor 接入
1. 打开 Cursor -> **Settings** -> **Models**；
2. 开启 **OpenAI API Key** 选项：
   - **API Key**：填入任意字符（例如 `workbuddy`）；
   - **Override OpenAI Base URL**：勾选并填入 `http://127.0.0.1:8317/v1`；
3. 在下方 Model List 中添加自定义模型名：
   - `cb/gpt-6.1-sol`
   - `cb/claude-sonnet-5-5`
   - `cb/gpt-6-astra`

### 6.2 Claude Code 终端接入
```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8317"
export ANTHROPIC_API_KEY="dummy-token"

# 启动 Claude Code 并指定国际区模型
claude --model cb/claude-sonnet-5-5
```

### 6.3 VS Code Continue / Cline / Roo Code 插件
- **Provider**：选择 `OpenAI Compatible`
- **Base URL**：`http://127.0.0.1:8317/v1`
- **API Key**：`dummy`
- **Model**：`cb/gpt-6.1-sol` 或 `cn/glm-5.3`
