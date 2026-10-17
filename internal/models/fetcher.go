package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
)

// DynamicFetcher 负责从 WorkBuddy 上游拉取最新上架模型列表
type DynamicFetcher struct {
	client *http.Client
}

// NewDynamicFetcher 实例化拉取器
func NewDynamicFetcher() *DynamicFetcher {
	return &DynamicFetcher{
		client: config.NewHTTPClient(25 * time.Second),
	}
}

type upstreamModelItem struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Vendor           string  `json:"vendor"`
	CreditMultiplier float64 `json:"creditMultiplier"`
	ContextLength    int     `json:"contextLength"`
	IsFree           bool    `json:"isFree"`
}

type upstreamModelResp struct {
	Code int `json:"code"`
	Data struct {
		Models []upstreamModelItem `json:"models"`
	} `json:"data"`
}

// FetchModelsByAccount 使用指定账号的凭据与 UA 获取其可见的模型目录
func (f *DynamicFetcher) FetchModelsByAccount(ctx context.Context, acc *auth.WorkBuddyAuth) ([]*ModelInfo, error) {
	if acc == nil {
		return nil, fmt.Errorf("account is nil")
	}

	apiURL := fmt.Sprintf("%s/v2/enterprises/personal/models", acc.BaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	req.Header.Set("Accept", "application/json, text/plain, */*")

	if acc.Realm == auth.RealmCN {
		req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
		req.Header.Set("Origin", "https://www.codebuddy.cn")
		req.Header.Set("Referer", "https://www.codebuddy.cn/")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("X-Product", "SaaS")
		req.Header.Set("X-Domain", "copilot.tencent.com")
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
		req.Header.Set("X-No-Department-Info", "1")
	} else {
		req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
		req.Header.Set("Origin", "https://www.codebuddy.ai")
		req.Header.Set("Referer", "https://www.codebuddy.ai/")
		req.Header.Set("X-Domain", acc.Domain())
		if acc.UserID != "" {
			req.Header.Set("X-User-Id", acc.UserID)
		}
	}

	client := config.NewHTTPClient(25*time.Second, acc.ProxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch models upstream status: %d", resp.StatusCode)
	}

	var parsed upstreamModelResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode models failed: %w", err)
	}

	var result []*ModelInfo
	for _, item := range parsed.Data.Models {
		result = append(result, &ModelInfo{
			ID:               item.ID,
			DisplayName:      item.Name,
			Vendor:           item.Vendor,
			Realm:            acc.Realm,
			CreditMultiplier: item.CreditMultiplier,
			ContextLength:    item.ContextLength,
			IsFree:           item.IsFree,
		})
	}

	return result, nil
}
