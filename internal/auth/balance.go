package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"workbuddy-ai-cliproxy/internal/config"
)

// BalanceChecker 积分资产与资源查询器
type BalanceChecker struct{}

// NewBalanceChecker 实例化查询器
func NewBalanceChecker() *BalanceChecker {
	return &BalanceChecker{}
}

// FetchBalance 查询账号当前剩余积分总额及各积分包明细 (支持代理、双区伪装与多字段兼容)
func (b *BalanceChecker) FetchBalance(ctx context.Context, acc *WorkBuddyAuth) (float64, []CreditPackage, error) {
	if acc == nil {
		return 0, nil, fmt.Errorf("account is nil")
	}

	client := config.NewHTTPClient(25*time.Second, acc.ProxyURL)

	// 1. 尝试主计费接口 /v2/billing/meter/get-user-resource
	credits, pkgs, err := b.fetchEndpoint(ctx, client, acc, fmt.Sprintf("%s/v2/billing/meter/get-user-resource", acc.BaseURL()))
	if err == nil && (credits > 0 || len(pkgs) > 0) {
		b.updateAccountBalance(acc, credits, pkgs)
		return credits, pkgs, nil
	}

	// 2. 若主接口未查到有效积分，尝试备用接口 /v2/billing/balance
	fallbackURL := fmt.Sprintf("%s/v2/billing/balance", acc.BaseURL())
	creditsFb, pkgsFb, errFb := b.fetchEndpoint(ctx, client, acc, fallbackURL)
	if errFb == nil && (creditsFb > 0 || len(pkgsFb) > 0) {
		b.updateAccountBalance(acc, creditsFb, pkgsFb)
		return creditsFb, pkgsFb, nil
	}

	// 3. 国内区特殊兜底：如果是 CN 账号，且此前已有积分记录，保留已有积分避免被抹为 0
	acc.mu.RLock()
	existingCredits := acc.Credits
	existingPkgs := acc.CreditPackages
	acc.mu.RUnlock()

	if existingCredits > 0 {
		return existingCredits, existingPkgs, nil
	}

	// 返回首次尝试的错误或解析结果
	if err != nil {
		return 0, nil, err
	}
	b.updateAccountBalance(acc, credits, pkgs)
	return credits, pkgs, nil
}

func (b *BalanceChecker) updateAccountBalance(acc *WorkBuddyAuth, credits float64, pkgs []CreditPackage) {
	acc.mu.Lock()
	acc.Credits = credits
	if len(pkgs) > 0 {
		acc.CreditPackages = pkgs
	}
	acc.mu.Unlock()

	_ = GetStore().Save(acc)
}

func (b *BalanceChecker) fetchEndpoint(ctx context.Context, client *http.Client, acc *WorkBuddyAuth, apiURL string) (float64, []CreditPackage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	if acc.Realm == RealmCN {
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
	} else {
		// Global / en
		req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
		req.Header.Set("Origin", "https://www.codebuddy.ai")
		req.Header.Set("Referer", "https://www.codebuddy.ai/")
		req.Header.Set("X-Domain", acc.Domain())
		if acc.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
		}
		if acc.UserID != "" {
			req.Header.Set("X-User-Id", acc.UserID)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request %s failed: %w", apiURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read response from %s failed: %w", apiURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[workbuddy] fetch balance %s status %d: %s", acc.ID, resp.StatusCode, string(bodyBytes))
		return 0, nil, fmt.Errorf("upstream status: %d", resp.StatusCode)
	}

	return parseFlexibleBalance(bodyBytes)
}

// parseFlexibleBalance 兼容解析上游各种变体结构的积分与包明细
func parseFlexibleBalance(body []byte) (float64, []CreditPackage, error) {
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &env); err != nil {
		return 0, nil, fmt.Errorf("unmarshal envelope failed: %w", err)
	}

	if env.Code != 0 {
		return 0, nil, fmt.Errorf("upstream code %d: %s", env.Code, env.Msg)
	}

	var dataMap map[string]any
	if err := json.Unmarshal(env.Data, &dataMap); err != nil {
		return 0, nil, nil
	}

	var totalCredits float64

	// 1. 尝试从常见字段提取总积分
	creditKeys := []string{"totalCredits", "total_credits", "remainingCredits", "remaining_credits", "credits", "balance", "availableCredits"}
	for _, k := range creditKeys {
		if val, ok := dataMap[k]; ok {
			if f, okFloat := val.(float64); okFloat {
				totalCredits = f
				break
			}
		}
	}

	// 2. 尝试解析 packages 列表
	var packages []CreditPackage
	var rawPackages any
	for _, k := range []string{"packages", "creditPackages", "credit_packages"} {
		if val, ok := dataMap[k]; ok {
			rawPackages = val
			break
		}
	}

	if rawPackages != nil {
		pkgBytes, _ := json.Marshal(rawPackages)
		var parsedPkgs []struct {
			PackageCode      string  `json:"packageCode"`
			PackageName      string  `json:"packageName"`
			RemainingCredits float64 `json:"remainingCredits"`
			InitialCredits   float64 `json:"initialCredits"`
			EffectiveTime    int64   `json:"effectiveTime"`
			ExpireTime       int64   `json:"expireTime"`
			Status           int     `json:"status"`
			// 兼容下划线
			PackageCodeAlt      string  `json:"package_code"`
			PackageNameAlt      string  `json:"package_name"`
			RemainingCreditsAlt float64 `json:"remaining_credits"`
			InitialCreditsAlt   float64 `json:"initial_credits"`
		}

		if err := json.Unmarshal(pkgBytes, &parsedPkgs); err == nil {
			var packageSum float64
			for _, p := range parsedPkgs {
				code := p.PackageCode
				if code == "" {
					code = p.PackageCodeAlt
				}
				name := p.PackageName
				if name == "" {
					name = p.PackageNameAlt
				}
				rem := p.RemainingCredits
				if rem == 0 {
					rem = p.RemainingCreditsAlt
				}
				initVal := p.InitialCredits
				if initVal == 0 {
					initVal = p.InitialCreditsAlt
				}

				packages = append(packages, CreditPackage{
					PackageCode:      code,
					PackageName:      name,
					RemainingCredits: rem,
					InitialCredits:   initVal,
					EffectiveTime:    p.EffectiveTime,
					ExpireTime:       p.ExpireTime,
					Status:           p.Status,
				})

				if rem > 0 {
					packageSum += rem
				}
			}

			// 如果顶层 totalCredits 为 0 但包明细有积分，则以包积分之和为准
			if totalCredits == 0 && packageSum > 0 {
				totalCredits = packageSum
			}
		}
	}

	return totalCredits, packages, nil
}
