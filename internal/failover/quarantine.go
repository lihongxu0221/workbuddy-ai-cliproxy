package failover

import (
	"fmt"
	"sync"
	"time"
)

type quarantineEntry struct {
	Until  time.Time
	Reason string
}

// QuarantineManager 处理账号与模型的故障隔离与熔断退避
type QuarantineManager struct {
	accountQuarantine map[string]quarantineEntry // accountID -> entry
	modelQuarantine   map[string]quarantineEntry // accountID:model -> entry
	mu                sync.RWMutex
}

var (
	defaultQuarantine *QuarantineManager
	onceQuarantine    sync.Once
)

// GetQuarantine 获取熔断管理器单例
func GetQuarantine() *QuarantineManager {
	onceQuarantine.Do(func() {
		defaultQuarantine = &QuarantineManager{
			accountQuarantine: make(map[string]quarantineEntry),
			modelQuarantine:   make(map[string]quarantineEntry),
		}
	})
	return defaultQuarantine
}

// QuarantineAccount 对账号实施全局隔离 (如 402 / 14018 额度耗尽)
func (m *QuarantineManager) QuarantineAccount(accountID string, duration time.Duration, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accountQuarantine[accountID] = quarantineEntry{
		Until:  time.Now().Add(duration),
		Reason: reason,
	}
}

// QuarantineModel 对 (账号, 模型) 维度实施局部降级隔离 (如 429 速率限制)
func (m *QuarantineManager) QuarantineModel(accountID, model string, duration time.Duration, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", accountID, model)
	m.modelQuarantine[key] = quarantineEntry{
		Until:  time.Now().Add(duration),
		Reason: reason,
	}
}

// IsQuarantined 检查账号或该模型是否处于隔离冷却中
func (m *QuarantineManager) IsQuarantined(accountID, model string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()

	// 1. 先检查全局账号隔离
	if entry, ok := m.accountQuarantine[accountID]; ok {
		if now.Before(entry.Until) {
			return true, entry.Reason
		}
	}

	// 2. 检查特定模型隔离
	if model != "" {
		key := fmt.Sprintf("%s:%s", accountID, model)
		if entry, ok := m.modelQuarantine[key]; ok {
			if now.Before(entry.Until) {
				return true, entry.Reason
			}
		}
	}

	return false, ""
}

// ClearQuarantine 手动解除指定账号的隔离
func (m *QuarantineManager) ClearQuarantine(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.accountQuarantine, accountID)
	prefix := accountID + ":"
	for k := range m.modelQuarantine {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			delete(m.modelQuarantine, k)
		}
	}
}
