package failover

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
	"workbuddy-ai-cliproxy/internal/config"
)

type sessionBinding struct {
	AccountID string `json:"account_id"`
	UpdatedAt int64  `json:"updated_at"`
}

// AffinityManager 软会话粘性管理器 (保护 Prompt Cache 命中率)
type AffinityManager struct {
	bindings map[string]sessionBinding
	filePath string
	mu       sync.RWMutex
}

var (
	defaultAffinity *AffinityManager
	onceAffinity    sync.Once
)

// GetAffinity 获取会话粘性管理器单例
func GetAffinity() *AffinityManager {
	onceAffinity.Do(func() {
		cfg := config.Get()
		filePath := cfg.ResolvePath("session-affinity.json")
		mgr := &AffinityManager{
			bindings: make(map[string]sessionBinding),
			filePath: filePath,
		}
		mgr.load()
		go mgr.cleanLoop()
		defaultAffinity = mgr
	})
	return defaultAffinity
}

func (m *AffinityManager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &m.bindings)
}

func (m *AffinityManager) save() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, err := json.Marshal(m.bindings)
	if err != nil {
		return
	}
	_ = os.WriteFile(m.filePath, data, 0644)
}

func (m *AffinityManager) cleanLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now().Unix()
		changed := false
		for k, v := range m.bindings {
			if now-v.UpdatedAt > 3600 { // 60 分钟过期
				delete(m.bindings, k)
				changed = true
			}
		}
		m.mu.Unlock()
		if changed {
			m.save()
		}
	}
}

// ComputeSessionKey 从请求报文计算稳定的会话哈希
func ComputeSessionKey(conversationID string, promptPrefix []byte) string {
	if conversationID != "" {
		return conversationID
	}
	if len(promptPrefix) == 0 {
		return ""
	}
	h := sha256.New()
	if len(promptPrefix) > 512 {
		h.Write(promptPrefix[:512])
	} else {
		h.Write(promptPrefix)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// GetBoundAccount 获取该会话已绑定的优先账号 ID
func (m *AffinityManager) GetBoundAccount(sessionKey string) string {
	if sessionKey == "" {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if b, ok := m.bindings[sessionKey]; ok {
		if time.Now().Unix()-b.UpdatedAt < 3600 {
			return b.AccountID
		}
	}
	return ""
}

// BindSession 建立或刷新会话与账号的绑定映射
func (m *AffinityManager) BindSession(sessionKey, accountID string) {
	if sessionKey == "" || accountID == "" {
		return
	}
	m.mu.Lock()
	m.bindings[sessionKey] = sessionBinding{
		AccountID: accountID,
		UpdatedAt: time.Now().Unix(),
	}
	m.mu.Unlock()
	m.save()
}
