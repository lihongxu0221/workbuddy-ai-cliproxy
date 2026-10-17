package auth

import (
	"testing"
)

func TestParseFlexibleBalance(t *testing.T) {
	// 1. 标准 camelCase totalCredits 与 packages
	json1 := `{
		"code": 0,
		"data": {
			"totalCredits": 280.5,
			"packages": [
				{
					"packageCode": "pkg-01",
					"packageName": "平台奖励积分 (Bonus Pack)",
					"remainingCredits": 30.0,
					"status": 1
				},
				{
					"packageCode": "pkg-02",
					"packageName": "每日签到奖励",
					"remainingCredits": 100.0,
					"status": 1
				}
			]
		}
	}`

	credits1, pkgs1, err1 := parseFlexibleBalance([]byte(json1))
	if err1 != nil {
		t.Fatalf("parse balance json1 failed: %v", err1)
	}
	if credits1 != 280.5 {
		t.Errorf("expected credits 280.5, got %f", credits1)
	}
	if len(pkgs1) != 2 {
		t.Errorf("expected 2 packages, got %d", len(pkgs1))
	}

	// 2. 顶层无 totalCredits，但包明细包含赠送积分(30)与签到积分(100) -> 必须正确自动累加为 130
	json2 := `{
		"code": 0,
		"data": {
			"packages": [
				{
					"package_code": "TCACA_bonus_30",
					"package_name": "成长中心每日赠送积分",
					"remaining_credits": 30.0,
					"status": 1
				},
				{
					"package_code": "TCACA_checkin_100",
					"package_name": "每日签到积分奖励",
					"remaining_credits": 100.0,
					"status": 1
				}
			]
		}
	}`

	credits2, pkgs2, err2 := parseFlexibleBalance([]byte(json2))
	if err2 != nil {
		t.Fatalf("parse balance json2 failed: %v", err2)
	}
	if credits2 != 130.0 {
		t.Errorf("expected credits 130.0 (sum of bonus & checkin), got %f", credits2)
	}
	if len(pkgs2) != 2 {
		t.Errorf("expected 2 packages, got %d", len(pkgs2))
	}

	// 3. Data 直接为纯包数组
	json3 := `{
		"code": 0,
		"data": [
			{
				"packageName": "国内签到 100 积分",
				"remainingCredits": 100,
				"status": 1
			}
		]
	}`

	credits3, pkgs3, err3 := parseFlexibleBalance([]byte(json3))
	if err3 != nil {
		t.Fatalf("parse balance json3 failed: %v", err3)
	}
	if credits3 != 100.0 {
		t.Errorf("expected credits 100.0, got %f", credits3)
	}
	if len(pkgs3) != 1 {
		t.Errorf("expected 1 package, got %d", len(pkgs3))
	}

	// 4. 下划线与字符串数值兼容 (如 "remaining_credits": "50.5")
	json4 := `{
		"code": 0,
		"data": {
			"checkin_credits": "100",
			"bonus_credits": "30",
			"total_credits": "130"
		}
	}`

	credits4, _, err4 := parseFlexibleBalance([]byte(json4))
	if err4 != nil {
		t.Fatalf("parse balance json4 failed: %v", err4)
	}
	if credits4 != 130.0 {
		t.Errorf("expected credits 130.0, got %f", credits4)
	}
}
