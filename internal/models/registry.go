package models

import (
	"sync"
	"workbuddy-ai-cliproxy/internal/auth"
)

// ModelInfo 统一模型元数据
type ModelInfo struct {
	ID               string     `json:"id"`
	DisplayName      string     `json:"displayName"`
	Vendor           string     `json:"vendor"`
	Realm            auth.Realm `json:"realm"`
	CreditMultiplier float64    `json:"creditMultiplier"`
	ContextLength    int        `json:"contextLength"`
	IsFree           bool       `json:"isFree"`
	Disabled         bool       `json:"disabled"`
}

// Registry 模型管理器
type Registry struct {
	models map[string]*ModelInfo
	mu     sync.RWMutex
}

var (
	defaultRegistry *Registry
	onceRegistry    sync.Once
)

// GetRegistry 获取全局模型注册表单例
func GetRegistry() *Registry {
	onceRegistry.Do(func() {
		r := &Registry{
			models: make(map[string]*ModelInfo),
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

	// 国内区/双区通用主流模型
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
		Realm:            auth.RealmGlobal,
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

// ListAll 获取已注册所有模型
func (r *Registry) ListAll() []*ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*ModelInfo, 0, len(r.models))
	for _, m := range r.models {
		list = append(list, m)
	}
	return list
}

// Get 根据模型 ID 检索模型元数据
func (r *Registry) Get(id string) *ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.models[id]
}

// SetDisabled 动态启用或下架模型
func (r *Registry) SetDisabled(id string, disabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.models[id]; ok {
		m.Disabled = disabled
	}
}

// MergeUpstream 将从上游接口抓取到的新模型动态合并到注册表
func (r *Registry) MergeUpstream(upstreamList []*ModelInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, up := range upstreamList {
		if existing, ok := r.models[up.ID]; ok {
			existing.DisplayName = up.DisplayName
			existing.CreditMultiplier = up.CreditMultiplier
			existing.ContextLength = up.ContextLength
			existing.IsFree = up.IsFree
		} else {
			r.models[up.ID] = up
		}
	}
}
