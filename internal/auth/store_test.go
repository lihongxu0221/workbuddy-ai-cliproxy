package auth

import (
	"testing"
	"time"
)

func TestParseAuthPayloadFormats(t *testing.T) {
	// 1. 标准 CodeBuddy 单账号
	codeBuddyJSON := `{
		"auth": {
			"accessToken": "eyTestAccessToken123",
			"refreshToken": "rfTestToken123",
			"expiresAt": 1893456000,
			"domain": "copilot.tencent.com"
		},
		"account": {
			"uid": "10086",
			"enterpriseId": "ent-999",
			"nickname": "腾讯云开发者"
		}
	}`

	accs1, err := ParseAuthPayload([]byte(codeBuddyJSON), "workbuddy.json", "/fake/path/workbuddy.json")
	if err != nil {
		t.Fatalf("parse codebuddy format failed: %v", err)
	}
	if len(accs1) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accs1))
	}
	acc1 := accs1[0]
	if acc1.Realm != RealmCN {
		t.Errorf("expected RealmCN, got %s", acc1.Realm)
	}
	if acc1.UserID != "10086" || acc1.EnterpriseID != "ent-999" {
		t.Errorf("expected uid 10086 and ent-999, got %s, %s", acc1.UserID, acc1.EnterpriseID)
	}
	if acc1.IsGlobal() {
		t.Errorf("expected IsGlobal() to be false")
	}

	// 2. 国际区 Global 单账号
	globalJSON := `{
		"type": "workbuddy",
		"access_token": "eyGlobalToken456",
		"refresh_token": "rfGlobalToken456",
		"expires_at": 1893456000,
		"realm": "global",
		"user_id": "intl-777",
		"nickname": "Global Developer"
	}`

	accs2, err := ParseAuthPayload([]byte(globalJSON), "workbuddy-global.json", "/fake/path/workbuddy-global.json")
	if err != nil {
		t.Fatalf("parse global format failed: %v", err)
	}
	if len(accs2) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accs2))
	}
	acc2 := accs2[0]
	if acc2.Realm != RealmGlobal || !acc2.IsGlobal() {
		t.Errorf("expected RealmGlobal, got %s", acc2.Realm)
	}
	if acc2.Domain() != "codebuddy.ai" {
		t.Errorf("expected domain codebuddy.ai, got %s", acc2.Domain())
	}

	// 3. 多账号结构 (accounts 包装)
	multiWrapped := `{
		"accounts": [
			{
				"auth": {
					"accessToken": "tok1",
					"domain": "copilot.tencent.com"
				},
				"account": { "uid": "cn-1" }
			},
			{
				"type": "workbuddy",
				"access_token": "tok2",
				"realm": "global",
				"user_id": "intl-2"
			}
		]
	}`

	accs3, err := ParseAuthPayload([]byte(multiWrapped), "multi.json", "")
	if err != nil {
		t.Fatalf("parse multi-wrapped failed: %v", err)
	}
	if len(accs3) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accs3))
	}
	if accs3[0].Realm != RealmCN || accs3[1].Realm != RealmGlobal {
		t.Errorf("expected CN and Global, got %s and %s", accs3[0].Realm, accs3[1].Realm)
	}

	// 4. 裸多账号数组
	bareArray := `[
		{
			"type": "workbuddy",
			"access_token": "tok-a",
			"realm": "cn",
			"user_id": "user-a"
		},
		{
			"type": "workbuddy",
			"access_token": "tok-b",
			"realm": "global",
			"user_id": "user-b"
		}
	]`

	accs4, err := ParseAuthPayload([]byte(bareArray), "array.json", "")
	if err != nil {
		t.Fatalf("parse bare array failed: %v", err)
	}
	if len(accs4) != 2 {
		t.Fatalf("expected 2 accounts from bare array, got %d", len(accs4))
	}
}

func TestAuthExpiryAndRealmSniff(t *testing.T) {
	acc := &WorkBuddyAuth{
		ExpiresAt: time.Now().Unix() + 200, // 距离过期小于 300 秒
	}
	if !acc.IsExpired() {
		t.Errorf("expected account to be expired within 300s buffer")
	}

	accFuture := &WorkBuddyAuth{
		ExpiresAt: time.Now().Unix() + 3600,
	}
	if accFuture.IsExpired() {
		t.Errorf("expected account not to be expired")
	}

	// SniffRealm 测试
	if SniffRealm("", "workbuddy-cn-01.json", "") != RealmCN {
		t.Errorf("expected RealmCN for workbuddy-cn-01.json")
	}
	if SniffRealm("", "workbuddy-intl-02.json", "") != RealmGlobal {
		t.Errorf("expected RealmGlobal for workbuddy-intl-02.json")
	}
	if SniffRealm("", "workbuddy-global.json", "") != RealmGlobal {
		t.Errorf("expected RealmGlobal for workbuddy-global.json")
	}
	if SniffRealm("https://codebuddy.ai", "sample.json", "") != RealmGlobal {
		t.Errorf("expected RealmGlobal for codebuddy.ai domain")
	}
}
