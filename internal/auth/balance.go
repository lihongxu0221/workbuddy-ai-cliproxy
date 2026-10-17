package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"workbuddy-ai-cliproxy/internal/config"
)

// BalanceChecker 积分资产与资源查询器
type BalanceChecker struct{}

// NewBalanceChecker 实例化查询器
func NewBalanceChecker() *BalanceChecker {
	return &BalanceChecker{}
}

// FetchBalance 查询账号当前剩余积分总额及各积分包明细 (支持国内区防爬标头、代理、包明细累加与赠送积分识别)
func (b *BalanceChecker) FetchBalance(ctx context.Context, acc *WorkBuddyAuth) (float64, []CreditPackage, error) {
	if acc == nil {
		return 0, nil, fmt.Errorf("account is nil")
	}

	client := config.NewHTTPClient(25*time.Second, acc.ProxyURL)

	// 1. 尝试主计费接口 /v2/billing/meter/get-user-resource
	primaryURL := fmt.Sprintf("%s/v2/billing/meter/get-user-resource", acc.BaseURL())
	credits, pkgs, err := b.fetchEndpoint(ctx, client, acc, primaryURL)
	if err == nil && (credits > 0 || len(pkgs) > 0) {
		b.updateAccountBalance(acc, credits, pkgs)
		return credits, pkgs, nil
	}

	// 2. 尝试备选接口 /v2/billing/balance
	fallbackURL := fmt.Sprintf("%s/v2/billing/balance", acc.BaseURL())
	creditsFb, pkgsFb, errFb := b.fetchEndpoint(ctx, client, acc, fallbackURL)
	if errFb == nil && (creditsFb > 0 || len(pkgsFb) > 0) {
		b.updateAccountBalance(acc, creditsFb, pkgsFb)
		return creditsFb, pkgsFb, nil
	}

	// 3. 兜底保护：如果上游返回失败但账号已有大于 0 的历史积分，保留历史积分不抹零
	acc.mu.RLock()
	existingCredits := acc.Credits
	existingPkgs := acc.CreditPackages
	acc.mu.RUnlock()

	if existingCredits > 0 {
		return existingCredits, existingPkgs, nil
	}

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
		return 0, nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return parseFlexibleBalance(bodyBytes)
}

// parseFlexibleBalance 深度解析上游各种变体结构的积分包与赠送/签到余额
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

	var totalCredits float64
	var packages []CreditPackage

	// 1. 如果 Data 是纯数组 (部分网关直接返回包列表 [ {...}, {...} ])
	var dataArr []any
	if err := json.Unmarshal(env.Data, &dataArr); err == nil && len(dataArr) > 0 {
		packages = extractPackagesFromSlice(dataArr)
		packageSum := sumPackageCredits(packages)
		if packageSum > 0 {
			totalCredits = packageSum
		}
		return totalCredits, packages, nil
	}

	// 2. 如果 Data 是 JSON 对象
	var dataMap map[string]any
	if err := json.Unmarshal(env.Data, &dataMap); err != nil {
		return 0, nil, nil
	}

	// 提取顶层总额与各种赠送/签到额度
	keysToInspect := []string{
		"totalCredits", "total_credits",
		"remainingCredits", "remaining_credits",
		"giftCredits", "gift_credits",
		"bonusCredits", "bonus_credits",
		"checkinCredits", "checkin_credits",
		"freeCredits", "free_credits",
		"trialCredits", "trial_credits",
		"availableCredits", "available_credits",
		"credits", "balance", "amount", "points",
	}

	for _, k := range keysToInspect {
		if val, ok := dataMap[k]; ok {
			f := toFloat(val)
			if f > 0 && f > totalCredits {
				totalCredits = f
			}
		}
	}

	// 寻找 packages 嵌套节点 (可能叫 packages, creditPackages, credit_packages, resourcePackages, list)
	pkgKeys := []string{"packages", "creditPackages", "credit_packages", "resourcePackages", "list", "items"}
	for _, pk := range pkgKeys {
		if val, ok := dataMap[pk]; ok {
			if sliceVal, okSlice := val.([]any); okSlice {
				pkgs := extractPackagesFromSlice(sliceVal)
				if len(pkgs) > 0 {
					packages = append(packages, pkgs...)
				}
			}
		}
	}

	// 寻找 userResource 嵌套节点
	if subRes, ok := dataMap["userResource"].(map[string]any); ok {
		for _, k := range keysToInspect {
			if val, okVal := subRes[k]; okVal {
				f := toFloat(val)
				if f > totalCredits {
					totalCredits = f
				}
			}
		}
		for _, pk := range pkgKeys {
			if val, okVal := subRes[pk]; okVal {
				if sliceVal, okSlice := val.([]any); okSlice {
					packages = append(packages, extractPackagesFromSlice(sliceVal)...)
				}
			}
		}
	}

	// 累加所有有效积分包余额
	packageSum := sumPackageCredits(packages)
	if packageSum > totalCredits {
		totalCredits = packageSum
	} else if totalCredits == 0 && packageSum > 0 {
		totalCredits = packageSum
	}

	return totalCredits, packages, nil
}

func extractPackagesFromSlice(slice []any) []CreditPackage {
	var pkgs []CreditPackage
	for _, item := range slice {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name := firstString(m, "packageName", "package_name", "name", "title", "packageType", "type")
		if name == "" {
			name = "积分包"
		}

		code := firstString(m, "packageCode", "package_code", "code", "id")
		rem := firstFloat(m, "remainingCredits", "remaining_credits", "remaining", "balance", "amount", "credits", "available")
		initVal := firstFloat(m, "initialCredits", "initial_credits", "initial", "total")
		effTime := int64(firstFloat(m, "effectiveTime", "effective_time", "start_time", "startTime"))
		expTime := int64(firstFloat(m, "expireTime", "expire_time", "end_time", "endTime"))
		status := int(firstFloat(m, "status", "state"))
		if status == 0 && rem > 0 {
			status = 1 // 只要剩余积分大于 0，标记为有效
		}

		pkgs = append(pkgs, CreditPackage{
			PackageCode:      code,
			PackageName:      name,
			RemainingCredits: rem,
			InitialCredits:   initVal,
			EffectiveTime:    effTime,
			ExpireTime:       expTime,
			Status:           status,
		})
	}
	return pkgs
}

func sumPackageCredits(packages []CreditPackage) float64 {
	var sum float64
	for _, p := range packages {
		if p.RemainingCredits > 0 {
			sum += p.RemainingCredits
		}
	}
	return math.Round(sum*1000) / 1000
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	default:
		return 0
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if val, ok := m[k]; ok {
			if s, okStr := val.(string); okStr && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if val, ok := m[k]; ok {
			if f := toFloat(val); f != 0 {
				return f
			}
		}
	}
	return 0
}
