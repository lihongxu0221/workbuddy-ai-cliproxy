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

// ExpertTrialActivator 国际区 250 积分 Expert 权益激活器
type ExpertTrialActivator struct {
	client *http.Client
}

// NewExpertTrialActivator 实例化激活器
func NewExpertTrialActivator() *ExpertTrialActivator {
	return &ExpertTrialActivator{
		client: config.NewHTTPClient(25*time.Second, ""),
	}
}

type trialResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Success       bool    `json:"success"`
		CreditAwarded float64 `json:"creditAwarded"`
	} `json:"data"`
}

// ActivateExpertTrial 为国际区新账号激活 250 积分 14 天 Expert 权益
func (t *ExpertTrialActivator) ActivateExpertTrial(ctx context.Context, acc *auth.WorkBuddyAuth) (float64, error) {
	if !acc.IsGlobal() {
		return 0, fmt.Errorf("account %s is not global realm", acc.ID)
	}

	apiURL := fmt.Sprintf("%s/v2/billing/trial", acc.BaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader([]byte(`{"trialType":"expert_14d"}`)))
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

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("trial request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed trialResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}

	if parsed.Code != 0 {
		return 0, fmt.Errorf("activate trial error code %d: %s", parsed.Code, parsed.Msg)
	}

	// 重新拉取一次余额以更新本地
	checker := auth.NewBalanceChecker()
	_, _, _ = checker.FetchBalance(ctx, acc)

	return parsed.Data.CreditAwarded, nil
}
