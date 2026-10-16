package failover

import (
	"sync"
	"workbuddy-ai-cliproxy/internal/config"
)

// AdmissionGate 单账号最大并发请求准入控制器
type AdmissionGate struct {
	slots sync.Map // map[string]chan struct{}
}

var (
	defaultGate *AdmissionGate
	onceGate    sync.Once
)

// GetGate 获取准入控制器单例
func GetGate() *AdmissionGate {
	onceGate.Do(func() {
		defaultGate = &AdmissionGate{}
	})
	return defaultGate
}

func (g *AdmissionGate) getSlotChan(accountID string) chan struct{} {
	maxSlots := config.Get().MaxConcurrency
	if maxSlots <= 0 {
		maxSlots = 2
	}

	val, _ := g.slots.LoadOrStore(accountID, make(chan struct{}, maxSlots))
	return val.(chan struct{})
}

// TryAcquire 尝试为指定账号获取执行槽位。若槽位已满则立即返回 false
func (g *AdmissionGate) TryAcquire(accountID string) bool {
	ch := g.getSlotChan(accountID)
	select {
	case ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release 释放指定账号的槽位
func (g *AdmissionGate) Release(accountID string) {
	val, ok := g.slots.Load(accountID)
	if !ok {
		return
	}
	ch := val.(chan struct{})
	select {
	case <-ch:
	default:
	}
}

// InFlightCount 获取指定账号当前的在飞并发请求数
func (g *AdmissionGate) InFlightCount(accountID string) int {
	val, ok := g.slots.Load(accountID)
	if !ok {
		return 0
	}
	ch := val.(chan struct{})
	return len(ch)
}
