package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"workbuddy-ai-cliproxy/internal/config"
)

// Refresher 处理 Token 过期续签与保活
type Refresher struct {
	client *http.Client
}

// NewRefresher 实例化刷新器
func NewRefresher() *Refresher {
	return &Refresher{
		client: config.NewHTTPClient(25 * time.Second),
	}
}

// RefreshIfExpired 若临近过期则自动触发续签并持久化写盘
func (r *Refresher) RefreshIfExpired(ctx context.Context, acc *WorkBuddyAuth) (*WorkBuddyAuth, error) {
	if !acc.IsExpired() {
		return acc, nil
	}
	return r.ForceRefresh(ctx, acc)
}

// ForceRefresh 强制执行 Token 续约，针对 CN 与 Global 采用不同上游规范
func (r *Refresher) ForceRefresh(ctx context.Context, acc *WorkBuddyAuth) (*WorkBuddyAuth, error) {
	if acc.RefreshToken == "" {
		return acc, fmt.Errorf("account %s has no refresh_token", acc.ID)
	}

	if acc.Realm == RealmCN {
		return r.refreshCN(ctx, acc)
	}
	return r.refreshGlobal(ctx, acc)
}

func (r *Refresher) refreshCN(ctx context.Context, acc *WorkBuddyAuth) (*WorkBuddyAuth, error) {
	apiURL := "https://copilot.tencent.com/v2/plugin/auth/token/refresh"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return acc, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	req.Header.Set("Origin", "https://www.codebuddy.cn")
	req.Header.Set("Referer", "https://www.codebuddy.cn/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Product", "SaaS")
	req.Header.Set("X-Domain", "copilot.tencent.com")
	req.Header.Set("X-Refresh-Token", acc.RefreshToken)
	req.Header.Set("X-Auth-Refresh-Source", "workbuddy")

	if acc.EnterpriseID != "" {
		req.Header.Set("X-Enterprise-Id", acc.EnterpriseID)
	} else {
		req.Header.Set("X-No-Enterprise-Id", "1")
	}
	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	} else {
		req.Header.Set("X-No-User-Id", "1")
	}
	req.Header.Set("X-No-Department-Info", "1")

	client := config.NewHTTPClient(25*time.Second, acc.ProxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return acc, fmt.Errorf("do cn refresh failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return acc, fmt.Errorf("cn refresh upstream returned status: %d (%s)", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int64  `json:"expiresIn"`
			Domain       string `json:"domain"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		return acc, fmt.Errorf("decode cn refresh failed: %w", err)
	}

	if env.Code != 0 || env.Data.AccessToken == "" {
		return acc, fmt.Errorf("cn refresh failed with code %d: %s", env.Code, env.Msg)
	}

	acc.mu.Lock()
	acc.AccessToken = env.Data.AccessToken
	if env.Data.RefreshToken != "" {
		acc.RefreshToken = env.Data.RefreshToken
	}
	if env.Data.ExpiresIn > 0 {
		acc.ExpiresAt = time.Now().Unix() + env.Data.ExpiresIn
	} else {
		acc.ExpiresAt = time.Now().Unix() + 86400*7
	}
	acc.mu.Unlock()

	_ = GetStore().Save(acc)
	return acc, nil
}

func (r *Refresher) refreshGlobal(ctx context.Context, acc *WorkBuddyAuth) (*WorkBuddyAuth, error) {
	refreshURL := fmt.Sprintf("%s/v2/auth/refresh", acc.BaseURL())
	reqBody := map[string]string{
		"refresh_token": acc.RefreshToken,
		"grant_type":    "refresh_token",
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
	req.Header.Set("Origin", "https://www.codebuddy.ai")
	req.Header.Set("Referer", "https://www.codebuddy.ai/")
	req.Header.Set("X-Domain", acc.Domain())
	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	}

	client := config.NewHTTPClient(25*time.Second, acc.ProxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return acc, fmt.Errorf("do global refresh failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return acc, fmt.Errorf("global refresh status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return acc, fmt.Errorf("decode global refresh failed: %w", err)
	}

	if parsed.Code != 0 || parsed.Data.AccessToken == "" {
		return acc, fmt.Errorf("global refresh failed with code %d: %s", parsed.Code, parsed.Message)
	}

	acc.mu.Lock()
	acc.AccessToken = parsed.Data.AccessToken
	if parsed.Data.RefreshToken != "" {
		acc.RefreshToken = parsed.Data.RefreshToken
	}
	if parsed.Data.ExpiresIn > 0 {
		acc.ExpiresAt = time.Now().Unix() + parsed.Data.ExpiresIn
	} else {
		acc.ExpiresAt = time.Now().Unix() + 86400*14
	}
	acc.mu.Unlock()

	_ = GetStore().Save(acc)
	return acc, nil
}
