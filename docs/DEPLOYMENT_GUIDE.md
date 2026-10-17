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

### 3.1 放置动态库文件与目录层级
将编译生成的二进制动态库文件拷贝至 CLIProxyAPI 宿主程序的 `plugins/` 目录中：
```text
CPA 根目录/ (例如 D:\AILocal\Tools\CLIProxyAPI\Bin\)
├── cli-proxy-api.exe (或 cli-proxy-api 二进制)
├── config.yaml       <-- 【核心配置文件就放在这里！】
├── auths/            <-- 存放账号凭据 JSON 的目录
└── plugins/          <-- 存放插件二进制动态库
    └── workbuddy.dll (Linux 为 workbuddy.so，macOS 为 workbuddy.dylib)
```

### 3.2 配置文件 `config.yaml` 放置与填法说明
> **注意**：本配置文件是 **CLIProxyAPI (CPA) 宿主程序的全局主配置文件**，放置在 `cli-proxy-api.exe` 同级目录下。
> 如果你的 CPA 之前已经在运行，**无需新建文件**，只需打开已有 `config.yaml` 将下方配置节点合并进去即可。
>
> ⚠️ **Docker 部署关键注意点**：
> Docker 部署时，**账号目录必须在挂载卷里**！
> 插件默认会自动探测 CPA 宿主已挂载 Volume 的 `auth-dir`（例如 `/root/.cli-proxy-api`），并在其下自动使用 `workbuddy/` 存储账号凭据、使用 `.codebuddy_data/` 存储会话与审计数据，启动时日志会打印绝对路径 `[workbuddy] auth dir: /root/.cli-proxy-api/workbuddy`，确保容器销毁重建后账号不丢失。

在 `config.yaml` 中完善如下节点：

```yaml
# 1. 服务端口与凭据目录 (Docker 部署时必须挂载数据卷)
port: 8317
auth-dir: "auths"

# 全局网络代理 (国内服务器访问海外 codebuddy.ai 必需)
# proxy-url: "socks5://172.19.0.1:10808"

# 2. 客户端调用使用的 API Key
api-keys:
  - "sk-workbuddy-local-test"

# 3. 远程管理配置 (CPAMP 面板连接用)
remote-management:
  allow-remote: true
  # 【管理密码说明】：
  # 这是 CPA 宿主的管理密码。当你使用 CPA Manager Plus (CPAMP) 面板连接 CPA 时，
  # 界面弹窗要求填写的 "Management Key" 就是这个值！
  # 首次可填入自定义明文密码，CPA 启动后会自动将其哈希加密存储。
  secret-key: "YourSuperSecretManagementKey"

# 4. 插件系统全局配置
plugins:
  enabled: true
  configs:
    workbuddyai:
      daily_bonus: true      # 国际区每日 30 积分自动打卡 (08:00/12:00/16:00/20:00 4-Tick)
      checkin_auto: true     # 国内区每日 100 积分自动签到 (09:00/21:00)
      max_concurrency: 2     # 单账号最大活跃并发数 (严防上游 429 封号，建议 2)
      cn_prefix: "cn/"       # 前缀路由定义 (请求 cn/glm-5.2 强制国内区)
      global_prefix: "cb/"   # 国际区路由前缀 (请求 cb/gpt-6-astra 或 en/gpt-6-astra 强制国际区)
      session_affinity: true # 启用软会话粘性以保护 Prompt Cache
      data_dir: ".codebuddy_data" # 插件持久化数据目录

    # 兼容别名配置
    workbuddy:
      daily_bonus: true
      checkin_auto: true
      max_concurrency: 2
      cn_prefix: "cn/"
      global_prefix: "cb/"
      session_affinity: true
      data_dir: ".codebuddy_data"
```

### 3.3 独立服务模式 (Standalone CLI) 部署
若宿主环境未安装 CGO 编译链，可直接以独立 HTTP 代理服务方式启动：
```bash
./workbuddy-server.exe -port 8318 -auth-dir ./auths -data-dir ./.codebuddy_data
```
- 直接提供 `/v1/chat/completions` 与 `/v1/models`，支持任意 OpenAI 客户端连接 `http://127.0.0.1:8318/v1`；
- 直接访问 `http://127.0.0.1:8318/ui` 即可打开内置完整管理面板。

---

## 4. 添加与管理 WorkBuddy 认证凭据

插件支持通过三种方式轻松接入并管理多账号：

### 4.1 方式一：Web 控制台一键授权（推荐）
1. 在 CPAMP 点击左侧 **「WorkBuddy 控制台」**；
2. 点击右上角 **「⚡ 扫码 / OAuth 登录」** 按钮；
3. 选择 **国际区 WorkBuddy AI** 或 **国内区 CodeBuddy**，点击 **获取授权链接**；
4. 浏览器将自动弹窗打开官方认证页，登录成功后插件后台自动拉取凭据、查询余额并加入账号池。

### 4.2 方式二：CPAMP「OAuth 登录」面板
1. 打开 CPAMP 左侧 **「OAuth 登录」** 导航菜单；
2. 列表中将自动出现 **WorkBuddy AI** 插件卡片；
3. 点击 **「开始 WorkBuddy AI 登录」**，在新标签页完成认证即可。

### 4.3 方式三：手动放置 JSON 凭据文件
在 CLIProxyAPI 的 `auths/` 目录中创建或导入凭据文件，支持标准单账号与多账号数组：

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
