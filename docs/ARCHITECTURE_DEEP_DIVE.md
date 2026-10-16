# WorkBuddy AI 插件核心架构深度解析：并发准入、会话粘性与容灾机制

本文档深入解析本插件在并发准入、软会话粘性、提示词缓存保护、故障转移及内存状态管理等核心工程模块的设计考量与实现算法。

---

## 1. 单账号并发准入控制 (Admission Gate)

### 1.1 为什么必须严格控制并发？
与普通的公有云 API（如 Azure OpenAI、OpenAI 官方 API）不同，WorkBuddy 面向个人开发者和桌面端协同。官方对账号的多并发调用极其敏感：
- 单账号并发 >= 3 时，极高概率触发反作弊封控（HTTP 429 降智或 Code 6004 网关拒绝）；
- 多个 IDE 客户端（如 Cursor、VS Code Copilot、Claude Code）同时开启索引或多任务时，容易瞬间激增并发。

### 1.2 准入槽位算法 (Slot Management)
插件内部为每个账号维护一个专属的信号量槽位：
```go
type AdmissionGate struct {
    slots sync.Map // map[string]chan struct{}
    maxSlots int   // 默认 2
}

func (g *AdmissionGate) TryAcquire(accountID string) bool {
    chAny, _ := g.slots.LoadOrStore(accountID, make(chan struct{}, g.maxSlots))
    ch := chAny.(chan struct{})
    select {
    case ch <- struct{}{}:
        return true
    default:
        return false // 槽位已满，立即失败并触发换号
    }
}

func (g *AdmissionGate) Release(accountID string) {
    if chAny, ok := g.slots.Load(accountID); ok {
        ch := chAny.(chan struct{})
        select {
        case <-ch:
        default:
        }
    }
}
```

### 1.3 零排队策略 (Fail-Fast & Reschedule)
当账号槽位已满时，插件**绝不挂起等待（No Queueing）**，而是立即执行调度决策：
1. 寻找同 Realm 中尚未满载且未处于冷却中的备用账号；
2. 如果找到备选账号，立即占槽并切流转发；
3. 如果当前 Realm 内所有账号的槽位均已占满，立即向客户端返回标准 HTTP 429 响应，并带上 `Retry-After: 1` 标头，保证客户端能感知后端真实拥塞状况，避免引发长连接超时。

---

## 2. 软会话粘性 (Soft Session Affinity) 与 Prompt Cache 经济学

### 2.1 Prompt Cache 的巨大经济价值
在代码补全与复杂多轮重构中，上下文（Prompt Tokens）往往高达数万至数十万 Token。
- **缓存未命中 (Cache Miss)**：消耗 100% 对应 Token 的费用/积分；
- **缓存命中 (Cache Hit)**：以 GPT-6.1 Sol / Claude 5.5 为例，缓存读取费率仅为原价的 10%（节约 90% 积分消耗）。
- 若每次请求随机 Round-Robin 调度账号，每个账号都必须重新加载上下文并计算 KV Cache，导致整体打卡领取的免费积分消耗速度加快 10 倍！

### 2.2 软会话绑定算法
1. **提取会话指纹**：
   从请求报文中提取客户端会话指纹。若客户端未传特定 Session ID，则计算首个 System Prompt 与 User Prompt 前 256 字符的 SHA-256 哈希作为 `SessionFingerprint`。
2. **粘性路由决策**：
   ```go
   func (a *AffinityManager) SelectAccount(sessionID string, realm string, eligibleAccounts []*Auth) *Auth {
       a.mu.RLock()
       boundAccountID, exists := a.bindings[sessionID]
       a.mu.RUnlock()

       if exists {
           for _, acc := range eligibleAccounts {
               if acc.ID == boundAccountID && !acc.IsQuarantined() && acc.HasFreeSlot() {
                   return acc // 完美命中粘性账号
               }
           }
       }

       // 未绑定，或原绑定账号繁忙/冷却：从候选列表中挑选最空闲账号
       selected := pickBestAvailable(eligibleAccounts)
       if selected != nil {
           a.mu.Lock()
           a.bindings[sessionID] = selected.ID // 重新绑定至新账号，防止反复横跳
           a.mu.Unlock()
       }
       return selected
   }
   ```
3. **软降级与持久化**：
   绑定关系定期持久化到本地 `.codebuddy_data/session-affinity.json`。服务重启后会话粘性不丢失；同时绑定关系具备 60 分钟滑动过期机制，避免长期积累无效映射。

---

## 3. 插件内智能故障转移 (In-Plugin Failover Pipeline)

### 3.1 为什么必须在插件内重试？
在 CLIProxyAPI 体系中，宿主如果直接把请求派发给 C-Shared 动态库，动态库函数一旦返回 Error，宿主将直接关闭客户端连接，而无法触发行内其它 Provider 候选。因此，插件内部必须自洽闭环实现 Retry Pipeline。

### 3.2 故障转移状态机
```
      [收到客户端请求]
              │
      ┌───────▼────────┐
      │ 会话粘性优选账号 │
      └───────┬────────┘
              │ (无槽或冷却?)
     [尝试同 Realm 备选账号]
              │
    ┌─────────▼─────────┐
    │ 成功获取准入槽位   │
    └─────────┬─────────┘
              │ 发送上游请求
      [检测上游 HTTP 状态]
       /        |         \
   [200 OK]  [429/6004]  [402/14018 额度耗尽]
      │         │                 │
  [开始流式]   [隔离该模型]      [隔离账号全局]
      │         │                 │
  [安全传输]   [释放槽位]        [释放槽位]
                └────────┬────────┘
                         │
             [同 Realm 递归重试下一账号]
             (最大允许重试 3 次)
```

### 3.3 关键边界保护
- **禁止跨 Realm 故障转移**：严禁将 CN 请求转移到 Global 账号，或将 Global 请求转移到 CN 账号。跨 Realm 会造成模型不可用与意料之外的计费混乱；
- **防重放保护 (No Replay on Partial Data)**：若上游已开始吐出数据（流式首包已成功发送给客户端），后续发生网络断开或 500 错误时，**绝不进行换号重试**（因为客户端已收到部分响应，重新发送会导致回答内容拼接错乱）。此时应直接平稳结束 SSE 流。

---

## 4. 内存管理与多线程安全

- 动态库长期驻留在宿主进程内存中；
- 所有映射表（`AdmissionGate`、`AffinityManager`、`QuarantineManager`）均采用读写锁（`sync.RWMutex`）或 `sync.Map` 保证并发无竞争；
- 避免在 C-Shared 导出符号边界创建不可控的孤儿 Goroutine，生命周期严格关联 Context；
- 内存指标与槽位使用状态通过 Management API 实时暴露，方便可视化面板直观调试。
