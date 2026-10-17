# WorkBuddy AI CLIProxyAPI Plugin Solution (workbuddy-ai-cliproxy)

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://golang.org)
[![CLIProxyAPI Compatibility](https://img.shields.io/badge/CLIProxyAPI-v6%20%7C%20v7%20%7C%20v8-success.svg)](https://github.com/router-for-me/CLIProxyAPI)

> **Enterprise-grade high-availability proxy plugin integrating Tencent WorkBuddy / CodeBuddy (CN & Global realms) into CLIProxyAPI (CPA).**
>
> Fully compatible with **CPA v6 / v7 / v8 plugin interfaces**, with native support for **both CN & EN (workbuddyai) dual platforms** and **multi-account resilience**. Transforms WorkBuddy multi-account pools into standard **OpenAI Chat Completions**, **Anthropic Claude Messages**, and **Google Gemini GenerateContent** endpoints, seamlessly powering Cursor, VS Code Copilot, Claude Code, Windsurf, Cline, Roo Code, and more.

---

## Table of Contents
- [Key Highlights](#key-highlights)
- [System Architecture](#system-architecture)
- [Supported Models](#supported-models)
- [Dual Realm & Credential Specs](#dual-realm--credential-specs)
- [Core Engineering Mechanisms](#core-engineering-mechanisms)
  - [1. Full Compatibility with CPA v6 / v7 / v8](#1-full-compatibility-with-cpa-v6--v7--v8)
  - [2. Global 30-Credit Daily Reward & CN Check-in](#2-global-30-credit-daily-reward--cn-check-in)
  - [3. Multi-Account Management & In-Plugin Resilient Failover](#3-multi-account-management--in-plugin-resilient-failover)
  - [4. Per-Account Concurrency Admission Gate](#4-per-account-concurrency-admission-gate)
  - [5. Soft Session Affinity & Prompt Cache Protection](#5-soft-session-affinity--prompt-cache-protection)
  - [6. Dynamic Model Manager & Realm Routing](#6-dynamic-model-manager--realm-routing)
- [Quick Start](#quick-start)
  - [1. Build Shared Library](#1-build-shared-library)
  - [2. Install into CLIProxyAPI](#2-install-into-cliproxyapi)
  - [3. Configuration](#3-configuration)
  - [4. Client Integration](#4-client-integration)
- [Embedded Management Web Console](#embedded-management-web-console)
- [License](#license)

---

## Key Highlights

1. **Full Compatibility with CPA v6 / v7 / v8**:
   - Standard C ABI exports (`cliproxy_plugin_init`, `cliproxyPluginCall`, `cliproxyPluginFree`, `cliproxyPluginShutdown`) for CPA v7 and v8.
   - Backward-compatible exports (`InitPlugin`, `RegisterRoutes`, `UnloadPlugin`) for CPA v6 and custom hosts.
   - Dynamic `schema_version` negotiation (`min(hostVersion, 6)`), eliminating version mismatch errors.
   - Standalone CLI HTTP proxy server mode (`:8318`), allowing deployment without CGO.
2. **Native Dual-Realm Integration (CN + Global / EN / workbuddyai)**:
   - Detects whether an account belongs to CN (`copilot.tencent.com` / `workbuddy.cn`) or Global (`workbuddy.ai` / `codebuddy.ai`), preventing unauthorized 401 redirect crashes on international credentials.
   - Dedicated model prefixes (`cn/` vs `cb/`, `en/`, `wb/`) ensure clean routing without naming collisions.
   - Forces upstream streaming (bypassing code 11101), rewrites Claude Code filter words, and injects maximum reasoning depth for Hunyuan 3.
3. **Multi-Account Matrix & In-Plugin Resilient Failover**:
   - Parses both individual JSON credentials and multi-account arrays in a single file (`Auths` expansion).
   - Transparently handles **HTTP 429**, **HTTP 402 / code 14018 (Credits exhausted)**, and **code 6004** errors by rotating to healthy accounts in the same realm without dropping client streams.
4. **Zero-Touch Daily Credit Harvest Engine**:
   - **Global Realm**: Automates daily report submission via `POST /v2/report` with 1.5s pacing. Verified next-day delivery of **30 credits/day per account** (`Bonus Pack`, 30-day validity), plus streak rewards (+50 at 14d, +150 at 28d) and 250-credit Expert trial.
   - **CN Realm**: Automated daily check-in delivering **100 credits/day + 500 credits/month**.
   - Idempotent heatmap detection with 4-tick spaced scheduler (`08:00`, `12:00`, `16:00`, `20:00`).
5. **Anti-Abuse Admission Control & Prompt Cache Affinity**:
   - Enforces a hard **maximum 2 in-flight requests per account** admission gate, preventing sudden bursts from triggering account bans.
   - **Soft Session Affinity** binds client conversations to specific accounts, maintaining high upstream Prompt Cache hit rates (reducing token billing by up to 90%) while degrading gracefully when congested.
6. **Full Bidirectional Protocol Translation & Embedded Dashboard**:
   - Supports standard OpenAI format, Anthropic Claude format (including thinking tokens), and Google Gemini format.
   - Embedded Web Console for real-time account status, credit balance monitoring, and model connectivity latency tests.

---

## System Architecture

```
Client (Cursor / VSCode / Claude Code)
                 │  OpenAI / Claude / Gemini APIs
                 ▼
     CLIProxyAPI Host Core (:8317)
                 │  C-Shared ABI (.dll / .so / .dylib)
                 ▼
╔══════════════════════════════════════════════════════════╗
║             WorkBuddy AI Proxy Plugin                    ║
║                                                          ║
║   ┌────────────────────┐   ┌─────────────────────────┐   ║
║   │  Model Registry    │   │ Protocol Translators    │   ║
║   │  & Realm Router    │   │ (OpenAI, Claude, Gemini)│   ║
║   └────────────────────┘   └─────────────────────────┘   ║
║                                                          ║
║   ┌────────────────────┐   ┌─────────────────────────┐   ║
║   │ Admission Gate     │   │ Soft Session Affinity   │   ║
║   │ (Max 2 Slots/Acc)  │   │ (Prompt Cache Hotness)  │   ║
║   └────────────────────┘   └─────────────────────────┘   ║
║                                                          ║
║   ┌──────────────────────────────────────────────────┐   ║
║   │ In-Plugin Failover & Quarantine Manager          │   ║
║   │ (429 Backoff, 14018 Quota Exhaustion, 6004 Retry)│   ║
║   └──────────────────────────────────────────────────┘   ║
║                                                          ║
║   ┌────────────────────┐   ┌─────────────────────────┐   ║
║   │ Growth Scheduler   │   │ Usage Audit Ledger      │   ║
║   │ (30 Cr/d Global,   │   │ (JSONL Credit & Tokens) │   ║
║   │  100 Cr/d CN)      │   │                         │   ║
║   └────────────────────┘   └─────────────────────────┘   ║
║                                                          ║
║   ┌──────────────────────────────────────────────────┐   ║
║   │ Embedded Web Dashboard (/v0/management/plugins)  │   ║
║   └──────────────────────────────────────────────────┘   ║
╚══════════════════════════════════════════════════════════╝
                 │                     │
          CN Gateway             Global Gateway
       copilot.tencent.com         workbuddy.ai
```

---

## Supported Models

Upstream models are dynamically queried from `/v2/enterprises/personal/models` and `/v3/config`:

- **OpenAI Family**: `gpt-6-astra`, `gpt-6.1-sol`, `gpt-6-luna` (Global realm)
- **Anthropic Family**: `claude-fable-5-1`, `claude-opus-5-5`, `claude-sonnet-5-5` (Global realm)
- **Google Family**: `gemini-3.8-flash` (Global realm)
- **DeepSeek Family**: `deepseek-v4`, `deepseek-v3`, `deepseek-r1` (CN & Global realms)
- **Tencent Family**: `hunyuan-turbo`, `hunyuan-pro` (CN realm)
- **Zhipu Family**: `glm-5.3` (CN realm)

---

## Dual Realm & Credential Specs

Place credentials in CLIProxyAPI's `auths/` folder as `workbuddy-<uid>-<realm>.json`:

```json
{
  "type": "workbuddy",
  "id": "workbuddy-user-intl-01",
  "realm": "global",
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "expires_at": 1740000000,
  "user_id": "10086",
  "email": "user@example.com",
  "nickname": "WorkBuddy-Intl",
  "credits": 280.0,
  "disabled": false
}
```

---

## Quick Start

### 1. Build Shared Library

```bash
# Windows (outputs bin/workbuddy.dll)
./build.bat

# Linux / macOS (outputs bin/workbuddy.so or bin/workbuddy.dylib)
make build
```

### 2. Install into CLIProxyAPI

Copy the compiled library to the `plugins/` directory:
```bash
cp bin/workbuddy.dll /path/to/cliproxyapi/plugins/
```

### 3. Configuration

The configuration file is **CLIProxyAPI's own `config.yaml`**, located in the root folder alongside `cli-proxy-api.exe`. If you already have an existing `config.yaml`, simply merge the following snippet into it:

> ⚠️ **Docker Deployment Notice**:
> During Docker containerized deployment, **the account directory must be inside a persistent volume**! The plugin automatically detects the volume-backed CPA `auth-dir` (such as `/root/.cli-proxy-api`) and uses the `workbuddy/` and `.codebuddy_data/` subdirectories under it, guaranteeing credentials and audit records persist across container recreation. The startup log outputs the exact path: `[workbuddy] auth dir: /root/.cli-proxy-api/workbuddy`.

```yaml
port: 8317
# Auth directory (must be on a persistent volume in Docker)
auth-dir: "auths"

# Outbound proxy (required for servers in China accessing codebuddy.ai)
# proxy-url: "socks5://172.19.0.1:10808"

# API keys for client access
api-keys:
  - "sk-workbuddy-local-test"

# Remote management configuration (used by CPAMP)
remote-management:
  allow-remote: true
  # CPA Management Key: Enter this key when connecting from CPA Manager Plus (CPAMP)
  secret-key: "YourSuperSecretManagementKey"

# Plugin configuration
plugins:
  enabled: true
  configs:
    workbuddyai:
      management_key: "YourSuperSecretManagementKey" # Auto-extracted and injected into Web UI headers
      daily_bonus: true      # Global 30-credit daily report (08:00/12:00/16:00/20:00 4-Tick)
      checkin_auto: true     # CN 100-credit daily check-in (09:00/21:00)
      max_concurrency: 2     # Max active concurrency per account (recommended: 2)
      cn_prefix: "cn/"       # CN realm prefix (e.g. cn/glm-5.2)
      global_prefix: "cb/"   # Global realm prefix (e.g. cb/gpt-6-astra or en/gpt-6-astra)
      session_affinity: true # Preserves Prompt Cache hotness
      data_dir: ".codebuddy_data" # Auto-resolved inside persistent volume

    # Compatibility alias
    workbuddy:
      management_key: "YourSuperSecretManagementKey"
      daily_bonus: true
      checkin_auto: true
      max_concurrency: 2
      cn_prefix: "cn/"
      global_prefix: "cb/"
      session_affinity: true
      data_dir: ".codebuddy_data"
```

### 4. Running the Service

- **Dynamic Shared Library Mode (inside CPA host)**:
  ```bash
  ./cli-proxy-api -config config.yaml
  ```
- **Standalone CLI Proxy Mode (no CGO required)**:
  ```bash
  ./workbuddy-server.exe -port 8318 -auth-dir ./auths -data-dir ./.codebuddy_data
  ```

---

## License
MIT License.
