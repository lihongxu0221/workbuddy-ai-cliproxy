package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// BalanceChecker 积分资产与资源查询器
type BalanceChecker struct {
	client *http.Client
}

// NewBalanceChecker 实例化查询器
func NewBalanceChecker() *BalanceChecker {
	return &BalanceChecker{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type userResourceResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TotalCredits float64         `json:"totalCredits"`
		Packages     []CreditPackage `json:"packages"`
	} `json:"data"`
}

// FetchBalance 查询账号当前剩余积分总额及各积分包明细
func (b *BalanceChecker) FetchBalance(ctx context.Context, acc *WorkBuddyAuth) (float64, []CreditPackage, error) {
	apiURL := fmt.Sprintf("%s/v2/billing/meter/get-user-resource", acc.BaseURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
	req.Header.Set("X-Domain", acc.Domain())
	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request balance failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("balance upstream status: %d", resp.StatusCode)
	}

	var res userResourceResp
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, nil, fmt.Errorf("decode balance failed: %w", err)
	}

	if res.Code != 0 {
		return 0, nil, fmt.Errorf("upstream error code %d: %s", res.Code, res.Msg)
	}

	acc.mu.Lock()
	acc.Credits = res.Data.TotalCredits
	acc.CreditPackages = res.Data.Packages
	acc.mu.Unlock()

	// 异步持久化更新到磁盘
	_ = GetStore().Save(acc)

	return res.Data.TotalCredits, res.Data.Packages, nil
}
