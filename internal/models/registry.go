package models

import (
	"context"
	"sync"
	"workbuddy-ai-cliproxy/internal/auth"
)

// ModelInfo 统一模型元数据
type ModelInfo struct {
	ID                string     `json:"id"`
	DisplayName       string     `json:"displayName"`
	Vendor            string     `json:"vendor"`
	Realm             auth.Realm `json:"realm"`
	CreditMultiplier  float64    `json:"creditMultiplier"`
	ContextLength     int        `json:"contextLength"`
	IsFree            bool       `json:"isFree"`
	Disabled          bool       `json:"disabled"`
	AvailableAccounts []string   `json:"availableAccounts,omitempty"`
}

// Registry 模型注册表与动态可用探测管理器
type Registry struct {
	models       map[string]*ModelInfo // 全局基础模板模型
	usableModels map[string]*ModelInfo // 当前已加载账号实际可见/可用的模型
	mu           sync.RWMutex
}

var (
	defaultRegistry *Registry
	onceRegistry    sync.Once
)

// GetRegistry 获取全局模型注册表单例
func GetRegistry() *Registry {
	onceRegistry.Do(func() {
		r := &Registry{
			models:       make(map[string]*ModelInfo),
			usableModels: make(map[string]*ModelInfo),
		}
		r.loadDefaults()
		defaultRegistry = r
	})
	return defaultRegistry
}

func (r *Registry) loadDefaults() {
	// 国际区官方旗舰模型
	r.registerStatic(&ModelInfo{
		ID:               "gpt-6-astra",
		DisplayName:      "OpenAI GPT-6 Astra (Flagship)",
		Vendor:           "OpenAI",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 1.5,
		ContextLength:    200000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "gpt-6.1-sol",
		DisplayName:      "OpenAI GPT-6.1 Sol (Balanced)",
		Vendor:           "OpenAI",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 1.0,
		ContextLength:    128000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "gpt-6-luna",
		DisplayName:      "OpenAI GPT-6 Luna (Fast)",
		Vendor:           "OpenAI",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 0.5,
		ContextLength:    128000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "claude-fable-5-1",
		DisplayName:      "Claude Fable 5.1 (Deep Reasoning)",
		Vendor:           "Anthropic",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 1.5,
		ContextLength:    200000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "claude-opus-5-5",
		DisplayName:      "Claude Opus 5.5",
		Vendor:           "Anthropic",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 1.2,
		ContextLength:    200000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "claude-sonnet-5-5",
		DisplayName:      "Claude Sonnet 5.5",
		Vendor:           "Anthropic",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 1.0,
		ContextLength:    200000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "gemini-3.8-flash",
		DisplayName:      "Google Gemini 3.8 Flash",
		Vendor:           "Google",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 0.3,
		ContextLength:    1000000,
		IsFree:           false,
	})

	// 国际区 DeepSeek 镜像模型
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v4.1-flash",
		DisplayName:      "DeepSeek V4.1 Flash",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 0.0,
		ContextLength:    128000,
		IsFree:           true,
	})
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v4-pro",
		DisplayName:      "DeepSeek V4 Pro",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 1.0,
		ContextLength:    1000000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v4",
		DisplayName:      "DeepSeek V4",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 0.8,
		ContextLength:    128000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v3",
		DisplayName:      "DeepSeek V3",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmGlobal,
		CreditMultiplier: 0.5,
		ContextLength:    64000,
		IsFree:           false,
	})

	// 国内区/双区通用主流模型
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v4.1-flash",
		DisplayName:      "DeepSeek V4.1 Flash (Free)",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.0,
		ContextLength:    128000,
		IsFree:           true,
	})
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v4",
		DisplayName:      "DeepSeek V4",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.8,
		ContextLength:    128000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v3",
		DisplayName:      "DeepSeek V3",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.5,
		ContextLength:    64000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v4-flash",
		DisplayName:      "DeepSeek V4 Flash (Free)",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.0,
		ContextLength:    64000,
		IsFree:           true,
	})
	r.registerStatic(&ModelInfo{
		ID:               "hunyuan-turbo",
		DisplayName:      "Tencent Hunyuan Turbo",
		Vendor:           "Tencent",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.6,
		ContextLength:    128000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "glm-5.3",
		DisplayName:      "Zhipu GLM-5.3",
		Vendor:           "Zhipu",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.8,
		ContextLength:    128000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "glm-5.2",
		DisplayName:      "GLM-5.2",
		Vendor:           "Zhipu",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.8,
		ContextLength:    1000000,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "glm-5.1",
		DisplayName:      "GLM-5.1",
		Vendor:           "Zhipu",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.6,
		ContextLength:    131072,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "glm-5v-turbo",
		DisplayName:      "GLM-5V Turbo",
		Vendor:           "Zhipu",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.6,
		ContextLength:    131072,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "kimi-k2.7",
		DisplayName:      "Kimi K2.7",
		Vendor:           "Moonshot",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.8,
		ContextLength:    262144,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "minimax-m3-pay",
		DisplayName:      "MiniMax M3",
		Vendor:           "MiniMax",
		Realm:            auth.RealmCN,
		CreditMultiplier: 0.8,
		ContextLength:    204800,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "hy3",
		DisplayName:      "Tencent Hy3 (Deep Reasoning)",
		Vendor:           "Tencent",
		Realm:            auth.RealmCN,
		CreditMultiplier: 1.0,
		ContextLength:    262144,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "hy3-preview",
		DisplayName:      "Tencent Hy3 Preview",
		Vendor:           "Tencent",
		Realm:            auth.RealmCN,
		CreditMultiplier: 1.0,
		ContextLength:    262144,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "hy3-preview-agent",
		DisplayName:      "Tencent Hy3 Preview Agent",
		Vendor:           "Tencent",
		Realm:            auth.RealmCN,
		CreditMultiplier: 1.0,
		ContextLength:    262144,
		IsFree:           false,
	})
	r.registerStatic(&ModelInfo{
		ID:               "deepseek-v4-pro",
		DisplayName:      "DeepSeek V4 Pro",
		Vendor:           "DeepSeek",
		Realm:            auth.RealmCN,
		CreditMultiplier: 1.0,
		ContextLength:    1000000,
		IsFree:           false,
	})
}

func (r *Registry) registerStatic(m *ModelInfo) {
	r.models[m.ID] = m
}

// SyncUsableModels 核心逻辑：遍历当前所有已配置账号，动态探查并聚合所有账号实际能用的模型
func (r *Registry) SyncUsableModels(ctx context.Context, accounts []*auth.WorkBuddyAuth) []*ModelInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 若未配置任何账号，返回空列表 (不展示任何未绑定账号的模型)
	if len(accounts) == 0 {
		r.usableModels = make(map[string]*ModelInfo)
		return nil
	}

	fetcher := NewDynamicFetcher()
	usable := make(map[string]*ModelInfo)

	for _, acc := range accounts {
		if acc.Disabled {
			continue
		}

		accIdentifier := acc.Nickname
		if accIdentifier == "" {
			accIdentifier = acc.ID
		}

		// 1. 尝试从上游动态接口抓取该账号实际可见的模型列表
		probedModels, err := fetcher.FetchModelsByAccount(ctx, acc)
		if err == nil && len(probedModels) > 0 {
			for _, m := range probedModels {
				// 动态同步到全局模型表，确保 CPA 宿主能全量识别
				r.models[m.ID] = m

				if existing, ok := usable[m.ID]; ok {
					if !containsString(existing.AvailableAccounts, accIdentifier) {
						existing.AvailableAccounts = append(existing.AvailableAccounts, accIdentifier)
					}
				} else {
					mCopy := *m
					mCopy.AvailableAccounts = []string{accIdentifier}
					usable[m.ID] = &mCopy
				}
			}

			// 补充该 Realm 已预置的核心模型 (确保如国际区 DeepSeek 镜像模型挂载完整)
			for _, def := range r.models {
				if def.Realm == acc.Realm {
					if _, ok := usable[def.ID]; !ok {
						mCopy := *def
						mCopy.AvailableAccounts = []string{accIdentifier}
						usable[def.ID] = &mCopy
					} else if !containsString(usable[def.ID].AvailableAccounts, accIdentifier) {
						usable[def.ID].AvailableAccounts = append(usable[def.ID].AvailableAccounts, accIdentifier)
					}
				}
			}
			continue
		}

		// 2. 兜底方案：依据账号所属 Realm，仅加载属于该区域的模型
		for _, m := range r.models {
			if m.Realm == acc.Realm {
				if existing, ok := usable[m.ID]; ok {
					if !containsString(existing.AvailableAccounts, accIdentifier) {
						existing.AvailableAccounts = append(existing.AvailableAccounts, accIdentifier)
					}
				} else {
					mCopy := *m
					mCopy.AvailableAccounts = []string{accIdentifier}
					usable[m.ID] = &mCopy
				}
			}
		}
	}

	r.usableModels = usable

	var result []*ModelInfo
	for _, m := range r.usableModels {
		result = append(result, m)
	}
	return result
}

// ListUsable 列出当前可用模型 (优先从 usableModels，若为空且有账号则即时同步)
func (r *Registry) ListUsable(accounts []*auth.WorkBuddyAuth) []*ModelInfo {
	r.mu.RLock()
	count := len(r.usableModels)
	r.mu.RUnlock()

	if count == 0 && len(accounts) > 0 {
		return r.SyncUsableModels(context.Background(), accounts)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ModelInfo
	for _, m := range r.usableModels {
		result = append(result, m)
	}
	return result
}

// ListAll 获取全局模版模型列表 (测试/静态分析备用)
func (r *Registry) ListAll() []*ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*ModelInfo, 0, len(r.models))
	for _, m := range r.models {
		list = append(list, m)
	}
	return list
}

// Get 根据模型 ID 检索模型元数据 (优先从 usableModels，兜底模版表)
func (r *Registry) Get(id string) *ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.usableModels[id]; ok {
		return m
	}
	return r.models[id]
}

// SetDisabled 动态启用或下架模型
func (r *Registry) SetDisabled(id string, disabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.usableModels[id]; ok {
		m.Disabled = disabled
	}
	if m, ok := r.models[id]; ok {
		m.Disabled = disabled
	}
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
