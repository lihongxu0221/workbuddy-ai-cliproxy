package models

import (
	"context"
	"testing"
	"workbuddy-ai-cliproxy/internal/auth"
)

func TestSyncUsableModelsFiltersByActiveAccounts(t *testing.T) {
	reg := GetRegistry()

	// 1. 无账号时，不展示任何模型
	emptyModels := reg.SyncUsableModels(context.Background(), nil)
	if len(emptyModels) != 0 {
		t.Errorf("expected 0 models when no accounts provided, got %d", len(emptyModels))
	}

	// 2. 只有国内区账号时，必须只返回国内区模型，绝不能返回未授权的国际区模型 (如 gpt-6-astra)
	cnAcc := &auth.WorkBuddyAuth{
		ID:       "workbuddy-cn-test",
		Realm:    auth.RealmCN,
		Nickname: "国内测试号",
	}

	cnOnlyModels := reg.SyncUsableModels(context.Background(), []*auth.WorkBuddyAuth{cnAcc})
	if len(cnOnlyModels) == 0 {
		t.Fatalf("expected CN models to be discovered for CN account")
	}

	for _, m := range cnOnlyModels {
		if m.Realm != auth.RealmCN {
			t.Errorf("expected only CN models when only CN account exists, but found: %s (%s)", m.ID, m.Realm)
		}
		if len(m.AvailableAccounts) == 0 || m.AvailableAccounts[0] != "国内测试号" {
			t.Errorf("expected AvailableAccounts to contain '国内测试号', got %v", m.AvailableAccounts)
		}
	}

	// 3. 只有国际区账号时，必须只返回国际区模型
	globalAcc := &auth.WorkBuddyAuth{
		ID:       "workbuddy-intl-test",
		Realm:    auth.RealmGlobal,
		Nickname: "海外主力号",
	}

	globalOnlyModels := reg.SyncUsableModels(context.Background(), []*auth.WorkBuddyAuth{globalAcc})
	if len(globalOnlyModels) == 0 {
		t.Fatalf("expected Global models to be discovered for Global account")
	}

	for _, m := range globalOnlyModels {
		if m.Realm != auth.RealmGlobal {
			t.Errorf("expected only Global models when only Global account exists, but found: %s (%s)", m.ID, m.Realm)
		}
		if len(m.AvailableAccounts) == 0 || m.AvailableAccounts[0] != "海外主力号" {
			t.Errorf("expected AvailableAccounts to contain '海外主力号', got %v", m.AvailableAccounts)
		}
	}

	// 4. 双区账号均存在时，聚合返回各自可用模型
	bothModels := reg.SyncUsableModels(context.Background(), []*auth.WorkBuddyAuth{cnAcc, globalAcc})
	var hasCN, hasGlobal bool
	for _, m := range bothModels {
		if m.Realm == auth.RealmCN {
			hasCN = true
		}
		if m.Realm == auth.RealmGlobal {
			hasGlobal = true
		}
	}
	if !hasCN || !hasGlobal {
		t.Errorf("expected both CN and Global models when both accounts present, got hasCN=%v, hasGlobal=%v", hasCN, hasGlobal)
	}
}
