package growth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
)

// CNCheckinExecutor 国内区每日签到执行器
type CNCheckinExecutor struct{}

// NewCNCheckinExecutor 实例化国内区签到器
func NewCNCheckinExecutor() *CNCheckinExecutor {
	return &CNCheckinExecutor{}
}

type checkinResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		CheckinSuccess     bool `json:"checkinSuccess"`
		RewardCredits      int  `json:"rewardCredits"`
		MonthlyBonusEarned bool `json:"monthlyBonusEarned"`
	} `json:"data"`
}

// Checkin 执行国内区每日 100 积分签到
func (c *CNCheckinExecutor) Checkin(ctx context.Context, acc *auth.WorkBuddyAuth) (int, error) {
	if acc.IsGlobal() {
		return 0, fmt.Errorf("account %s is not CN realm", acc.ID)
	}

	client := config.NewHTTPClient(25*time.Second, acc.ProxyURL)
	apiURL := fmt.Sprintf("%s/v2/checkin", acc.BaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	req.Header.Set("Origin", "https://www.codebuddy.cn")
	req.Header.Set("Referer", "https://www.codebuddy.cn/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Product", "SaaS")
	req.Header.Set("X-Domain", "copilot.tencent.com")

	if acc.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	} else {
		req.Header.Set("X-No-Authorization", "1")
	}

	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	} else {
		req.Header.Set("X-No-User-Id", "1")
	}

	if acc.EnterpriseID != "" {
		req.Header.Set("X-Enterprise-Id", acc.EnterpriseID)
	} else {
		req.Header.Set("X-No-Enterprise-Id", "1")
	}

	if acc.RefreshToken != "" {
		req.Header.Set("X-Refresh-Token", acc.RefreshToken)
	}
	req.Header.Set("X-No-Department-Info", "1")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("checkin request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed checkinResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}

	if parsed.Code != 0 {
		return 0, fmt.Errorf("checkin error code %d: %s", parsed.Code, parsed.Msg)
	}

	reward := parsed.Data.RewardCredits
	if reward > 0 {
		acc.Credits += float64(reward)
		acc.CreditPackages = append(acc.CreditPackages, auth.CreditPackage{
			PackageName:      "每日签到奖励",
			RemainingCredits: float64(reward),
			InitialCredits:   float64(reward),
			Status:           1,
			ExpireTime:       time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
		})
		_ = auth.GetStore().Save(acc)
	}

	// 签到完成后拉取一次全量余额以同步最新账单
	checker := auth.NewBalanceChecker()
	_, _, _ = checker.FetchBalance(ctx, acc)

	return reward, nil
}
