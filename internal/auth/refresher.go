package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Refresher 处理 Token 过期续签与保活
type Refresher struct {
	client *http.Client
}

// NewRefresher 实例化刷新器
func NewRefresher() *Refresher {
	return &Refresher{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
	GrantType    string `json:"grant_type"`
}

type refreshResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	} `json:"data"`
}

// RefreshIfExpired 若临近过期则自动触发续签并持久化写盘
func (r *Refresher) RefreshIfExpired(ctx context.Context, acc *WorkBuddyAuth) (*WorkBuddyAuth, error) {
	if !acc.IsExpired() {
		return acc, nil
	}
	return r.ForceRefresh(ctx, acc)
}

// ForceRefresh 强制执行 Token 续约
func (r *Refresher) ForceRefresh(ctx context.Context, acc *WorkBuddyAuth) (*WorkBuddyAuth, error) {
	if acc.RefreshToken == "" {
		return acc, fmt.Errorf("account %s has no refresh_token", acc.ID)
	}

	refreshURL := fmt.Sprintf("%s/v2/auth/refresh", acc.BaseURL())
	reqBody := refreshReq{
		RefreshToken: acc.RefreshToken,
		GrantType:    "refresh_token",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return acc, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return acc, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
	req.Header.Set("X-Domain", acc.Domain())
	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return acc, fmt.Errorf("do refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return acc, fmt.Errorf("refresh upstream returned status: %d", resp.StatusCode)
	}

	var parsed refreshResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return acc, fmt.Errorf("decode refresh response failed: %w", err)
	}

	if parsed.Code != 0 || parsed.Data.AccessToken == "" {
		return acc, fmt.Errorf("refresh failed with code %d: %s", parsed.Code, parsed.Message)
	}

	acc.mu.Lock()
	acc.AccessToken = parsed.Data.AccessToken
	if parsed.Data.RefreshToken != "" {
		acc.RefreshToken = parsed.Data.RefreshToken
	}
	expiresIn := parsed.Data.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 86400
	}
	acc.ExpiresAt = time.Now().Unix() + expiresIn
	acc.mu.Unlock()

	// 异步持久化保存更新后的凭据
	_ = GetStore().Save(acc)
	return acc, nil
}
