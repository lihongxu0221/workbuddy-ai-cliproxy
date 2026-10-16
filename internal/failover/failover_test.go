package failover

import (
	"testing"
	"time"
)

func TestAdmissionGateMaxConcurrency(t *testing.T) {
	gate := GetGate()
	accountID := "test-acc-gate"

	// 1. 连续获取 2 个槽位应该成功
	if !gate.TryAcquire(accountID) {
		t.Fatalf("first slot acquire failed")
	}
	if !gate.TryAcquire(accountID) {
		t.Fatalf("second slot acquire failed")
	}

	// 2. 第 3 个槽位应该被拒绝
	if gate.TryAcquire(accountID) {
		t.Fatalf("third slot acquire should be rejected")
	}

	if gate.InFlightCount(accountID) != 2 {
		t.Errorf("expected 2 in-flight slots, got %d", gate.InFlightCount(accountID))
	}

	// 3. 释放 1 个槽位后应该可以再次获取
	gate.Release(accountID)
	if gate.InFlightCount(accountID) != 1 {
		t.Errorf("expected 1 in-flight slot after release, got %d", gate.InFlightCount(accountID))
	}

	if !gate.TryAcquire(accountID) {
		t.Fatalf("acquire after release should succeed")
	}

	// 清理
	gate.Release(accountID)
	gate.Release(accountID)
	if gate.InFlightCount(accountID) != 0 {
		t.Errorf("expected 0 in-flight slots, got %d", gate.InFlightCount(accountID))
	}
}

func TestAffinityManager(t *testing.T) {
	affinity := GetAffinity()
	sessionKey := "sess-user-123"
	accountID := "workbuddy-cn-primary"

	affinity.BindSession(sessionKey, accountID)
	if bound := affinity.GetBoundAccount(sessionKey); bound != accountID {
		t.Errorf("expected bound account %s, got %s", accountID, bound)
	}
}

func TestQuarantineManager(t *testing.T) {
	quarantine := GetQuarantine()
	accountID := "quarantine-acc-1"
	model := "hy3"

	// 初始状态清理
	// 模型隔离 (例如 429 限流)
	quarantine.QuarantineModel(accountID, model, 1*time.Second, "429 rate limit")
	if isQ, reason := quarantine.IsQuarantined(accountID, model); !isQ {
		t.Fatalf("expected account to be quarantined for model %s (reason: %s)", model, reason)
	}
	// 其他模型不受影响
	if isQ, _ := quarantine.IsQuarantined(accountID, "glm-5.2"); isQ {
		t.Fatalf("other model should not be quarantined")
	}

	// 全局账号隔离 (例如 14018 额度耗尽)
	quarantine.QuarantineAccount(accountID, 1*time.Second, "14018 credit exhausted")
	if isQ, reason := quarantine.IsQuarantined(accountID, "glm-5.2"); !isQ {
		t.Fatalf("expected whole account to be quarantined (reason: %s)", reason)
	}
}
