package growth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
)

// CNCheckinExecutor 国内区每日签到执行器
type CNCheckinExecutor struct {
	client *http.Client
}

// NewCNCheckinExecutor 实例化国内区签到器
func NewCNCheckinExecutor() *CNCheckinExecutor {
	return &CNCheckinExecutor{
		client: &http.Client{Timeout: 15 * time.Second},
	}
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

	apiURL := fmt.Sprintf("%s/v2/checkin", acc.BaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
	req.Header.Set("X-Domain", acc.Domain())
	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	}

	resp, err := c.client.Do(req)
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

	return parsed.Data.RewardCredits, nil
}
