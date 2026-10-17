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

// FetchBalance 查询账号当前剩余积分总额及各积分包明细 (完整对接官方客户端资源体系，提取套餐类型与赠送/签到积分)
func (b *BalanceChecker) FetchBalance(ctx context.Context, acc *WorkBuddyAuth) (float64, []CreditPackage, error) {
	if acc == nil {
		return 0, nil, fmt.Errorf("account is nil")
	}

	client := config.NewHTTPClient(25*time.Second, acc.ProxyURL)

	// 1. 优先调用官方客户端使用的综合资源概要接口 /billing/meter/get-user-resource-summary
	summaryURL := fmt.Sprintf("%s/billing/meter/get-user-resource-summary", acc.BaseURL())
	plan, creditsSummary, pkgsSummary, errSummary := b.fetchSummaryEndpoint(ctx, client, acc, summaryURL)

	// 2. 同时调用 /billing/meter/get-user-resource-free-packages 获取赠送与签到详细包 (带今日 SlicePeriod 范围)
	freeURL := fmt.Sprintf("%s/billing/meter/get-user-resource-free-packages", acc.BaseURL())
	creditsFree, pkgsFree, errFree := b.fetchPackagesEndpoint(ctx, client, acc, freeURL, true)

	// 3. 同时调用 /billing/meter/get-user-resource-paid-packages 获取付费包
	paidURL := fmt.Sprintf("%s/billing/meter/get-user-resource-paid-packages", acc.BaseURL())
	creditsPaid, pkgsPaid, _ := b.fetchPackagesEndpoint(ctx, client, acc, paidURL, false)

	// 合并所有积分包
	allPkgs := mergePackages(pkgsSummary, pkgsFree)
	allPkgs = mergePackages(allPkgs, pkgsPaid)

	finalCredits := math.Max(creditsSummary, creditsFree+creditsPaid)
	if finalCredits == 0 && len(allPkgs) > 0 {
		finalCredits = sumPackageCredits(allPkgs)
	}

	log.Printf("[workbuddy] balance probed for %s (%s): plan=%s summary=%.2f free=%.2f paid=%.2f final=%.2f pkgs=%d",
		acc.ID, acc.Realm, plan, creditsSummary, creditsFree, creditsPaid, finalCredits, len(allPkgs))

	if (errSummary == nil || errFree == nil) && (finalCredits > 0 || len(allPkgs) > 0 || plan != "") {
		b.updateAccountBalanceWithPlan(acc, finalCredits, allPkgs, plan)
		return finalCredits, allPkgs, nil
	}

	// 4. 尝试旧版兼容接口 /v2/billing/meter/get-user-resource
	primaryURL := fmt.Sprintf("%s/v2/billing/meter/get-user-resource", acc.BaseURL())
	creditsOld, pkgsOld, errOld := b.fetchOldEndpoint(ctx, client, acc, primaryURL)
	if errOld == nil && (creditsOld > 0 || len(pkgsOld) > 0) {
		b.updateAccountBalanceWithPlan(acc, creditsOld, pkgsOld, plan)
		return creditsOld, pkgsOld, nil
	}

	// 5. 兜底保护：如果上游暂时返回失败，但账号已有历史积分，保留已有积分不抹零
	acc.mu.RLock()
	existingCredits := acc.Credits
	existingPkgs := acc.CreditPackages
	existingPlan := acc.Plan
	acc.mu.RUnlock()

	if existingCredits > 0 {
		return existingCredits, existingPkgs, nil
	}

	if plan != "" && acc.Plan == "" {
		acc.mu.Lock()
		acc.Plan = plan
		acc.mu.Unlock()
		_ = GetStore().Save(acc)
	}

	if existingPlan != "" && acc.Plan == "" {
		acc.Plan = existingPlan
	}

	if errSummary != nil && errFree != nil {
		return 0, nil, errSummary
	}
	b.updateAccountBalanceWithPlan(acc, finalCredits, allPkgs, plan)
	return finalCredits, allPkgs, nil
}

func (b *BalanceChecker) updateAccountBalanceWithPlan(acc *WorkBuddyAuth, credits float64, pkgs []CreditPackage, plan string) {
	acc.mu.Lock()
	acc.Credits = math.Round(credits*100) / 100
	if len(pkgs) > 0 {
		acc.CreditPackages = pkgs
	}
	if plan != "" {
		acc.Plan = plan
	} else if acc.Plan == "" {
		if acc.Realm == RealmCN {
			acc.Plan = "体验版"
		} else {
			acc.Plan = "免费版"
		}
	}
	acc.mu.Unlock()

	_ = GetStore().Save(acc)
}

func (b *BalanceChecker) fetchSummaryEndpoint(ctx context.Context, client *http.Client, acc *WorkBuddyAuth, apiURL string) (string, float64, []CreditPackage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", 0, nil, err
	}
	applyHeaders(req, acc)
	req.Header.Set("Accept-Language", "zh")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, nil, fmt.Errorf("request summary failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[workbuddy] fetch summary %s status %d: %s", acc.ID, resp.StatusCode, string(bodyBytes))
		return "", 0, nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return parseSummaryResponse(bodyBytes)
}

func (b *BalanceChecker) fetchPackagesEndpoint(ctx context.Context, client *http.Client, acc *WorkBuddyAuth, apiURL string, isFree bool) (float64, []CreditPackage, error) {
	// 严格遵循客户端 buildSlicePeriodRange 逻辑构建今日切片范围 (带 00:00:00 ~ 23:59:59)
	cst := time.FixedZone("CST", 8*3600)
	nowCST := time.Now().In(cst)
	startDay := nowCST.Format("2006-01-02") + " 00:00:00"
	endDay := nowCST.Format("2006-01-02") + " 23:59:59"

	payloadMap := map[string]any{
		"PageNumber": 1,
		"PageSize":   200,
		"Status":     []int{1, 2},
	}

	if isFree {
		payloadMap["SlicePeriodStartTime"] = startDay
		payloadMap["SlicePeriodEndTime"] = endDay
	} else {
		payloadMap["NeedRenewInfo"] = true
	}

	payloadBytes, _ := json.Marshal(payloadMap)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, nil, err
	}
	applyHeaders(req, acc)
	req.Header.Set("Accept-Language", "zh")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return parsePackagesResponse(bodyBytes)
}

func (b *BalanceChecker) fetchOldEndpoint(ctx context.Context, client *http.Client, acc *WorkBuddyAuth, apiURL string) (float64, []CreditPackage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, nil, err
	}
	applyHeaders(req, acc)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return parseFlexibleBalance(bodyBytes)
}

func applyHeaders(req *http.Request, acc *WorkBuddyAuth) {
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
		req.Header.Set("User-Agent", "WorkBuddyAI/5.8.4 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
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
}

func parseSummaryResponse(body []byte) (string, float64, []CreditPackage, error) {
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Code != 0 {
		return "", 0, nil, fmt.Errorf("summary envelope invalid")
	}

	var data struct {
		SubscriptionPackageName string `json:"SubscriptionPackageName"`
		SubscriptionPackageCode string `json:"SubscriptionPackageCode"`
		IsPaidUser              bool   `json:"IsPaidUser"`
		Packages                []struct {
			PackageCode         string `json:"PackageCode"`
			PackageName         string `json:"PackageName"`
			CycleTotalCapacity  any    `json:"CycleTotalCapacity"`
			CycleRemainCapacity any    `json:"CycleRemainCapacity"`
			CycleUsedCapacity   any    `json:"CycleUsedCapacity"`
		} `json:"Packages"`
	}

	_ = json.Unmarshal(env.Data, &data)

	plan := strings.TrimSpace(data.SubscriptionPackageName)
	if plan == "" {
		if data.IsPaidUser {
			plan = "Pro"
		}
	}

	var totalRemain float64
	var pkgs []CreditPackage
	for _, p := range data.Packages {
		name := p.PackageName
		if name == "" {
			name = resolvePackageDisplayName(p.PackageCode)
		}
		rem := toFloat(p.CycleRemainCapacity)
		initVal := toFloat(p.CycleTotalCapacity)
		if rem > 0 {
			totalRemain += rem
		}
		pkgs = append(pkgs, CreditPackage{
			PackageCode:      p.PackageCode,
			PackageName:      name,
			RemainingCredits: rem,
			InitialCredits:   initVal,
			Status:           1,
		})
	}

	return plan, math.Round(totalRemain*100) / 100, pkgs, nil
}

func parsePackagesResponse(body []byte) (float64, []CreditPackage, error) {
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Code != 0 {
		return 0, nil, fmt.Errorf("packages envelope invalid")
	}

	// 兼容任意首字母大写/小写与字符串数值
	var data struct {
		Accounts []struct {
			ResourceId                 string `json:"ResourceId"`
			PackageCode                string `json:"PackageCode"`
			PackageName                string `json:"PackageName"`
			CapacityType               any    `json:"CapacityType"`
			CycleCapacityRemainPrecise any    `json:"CycleCapacityRemainPrecise"`
			CycleCapacitySizePrecise   any    `json:"CycleCapacitySizePrecise"`
			SlicePeriodUsageDetails    []struct {
				SlicePeriodCapacityRemainPrecise any `json:"SlicePeriodCapacityRemainPrecise"`
				SlicePeriodCapacitySizePrecise   any `json:"SlicePeriodCapacitySizePrecise"`
			} `json:"SlicePeriodUsageDetails"`
		} `json:"Accounts"`
		// 备用小写
		AccountsAlt []struct {
			ResourceId                 string `json:"resourceId"`
			PackageCode                string `json:"packageCode"`
			PackageName                string `json:"packageName"`
			CapacityType               any    `json:"capacityType"`
			CycleCapacityRemainPrecise any    `json:"cycleCapacityRemainPrecise"`
			CycleCapacitySizePrecise   any    `json:"cycleCapacitySizePrecise"`
			SlicePeriodUsageDetails    []struct {
				SlicePeriodCapacityRemainPrecise any `json:"slicePeriodCapacityRemainPrecise"`
				SlicePeriodCapacitySizePrecise   any `json:"slicePeriodCapacitySizePrecise"`
			} `json:"slicePeriodUsageDetails"`
		} `json:"accounts"`
	}

	_ = json.Unmarshal(env.Data, &data)

	var totalRemain float64
	var pkgs []CreditPackage

	list := data.Accounts
	if len(list) == 0 && len(data.AccountsAlt) > 0 {
		for _, a := range data.AccountsAlt {
			list = append(list, struct {
				ResourceId                 string `json:"ResourceId"`
				PackageCode                string `json:"PackageCode"`
				PackageName                string `json:"PackageName"`
				CapacityType               any    `json:"CapacityType"`
				CycleCapacityRemainPrecise any    `json:"CycleCapacityRemainPrecise"`
				CycleCapacitySizePrecise   any    `json:"CycleCapacitySizePrecise"`
				SlicePeriodUsageDetails    []struct {
					SlicePeriodCapacityRemainPrecise any `json:"SlicePeriodCapacityRemainPrecise"`
					SlicePeriodCapacitySizePrecise   any `json:"SlicePeriodCapacitySizePrecise"`
				} `json:"SlicePeriodUsageDetails"`
			}{
				ResourceId:                 a.ResourceId,
				PackageCode:                a.PackageCode,
				PackageName:                a.PackageName,
				CapacityType:               a.CapacityType,
				CycleCapacityRemainPrecise: a.CycleCapacityRemainPrecise,
				CycleCapacitySizePrecise:   a.CycleCapacitySizePrecise,
			})
		}
	}

	for _, a := range list {
		rem := toFloat(a.CycleCapacityRemainPrecise)
		initVal := toFloat(a.CycleCapacitySizePrecise)

		// 核心关键：如果是切片配额 (CapacityType == 4 或包含 SlicePeriodUsageDetails)，优先从 Slice 节点读取真实剩余
		if len(a.SlicePeriodUsageDetails) > 0 {
			sliceRem := toFloat(a.SlicePeriodUsageDetails[0].SlicePeriodCapacityRemainPrecise)
			sliceInit := toFloat(a.SlicePeriodUsageDetails[0].SlicePeriodCapacitySizePrecise)
			if sliceRem > 0 || rem == 0 {
				rem = sliceRem
				initVal = sliceInit
			}
		}

		name := a.PackageName
		if name == "" {
			name = resolvePackageDisplayName(a.PackageCode)
		}

		if rem > 0 {
			totalRemain += rem
		}

		pkgs = append(pkgs, CreditPackage{
			PackageCode:      a.PackageCode,
			PackageName:      name,
			RemainingCredits: rem,
			InitialCredits:   initVal,
			Status:           1,
		})
	}

	return math.Round(totalRemain*100) / 100, pkgs, nil
}

func mergePackages(primary, secondary []CreditPackage) []CreditPackage {
	seen := make(map[string]bool)
	var merged []CreditPackage
	for _, p := range primary {
		if !seen[p.PackageCode] {
			seen[p.PackageCode] = true
			merged = append(merged, p)
		}
	}
	for _, p := range secondary {
		if !seen[p.PackageCode] {
			seen[p.PackageCode] = true
			merged = append(merged, p)
		}
	}
	return merged
}

func resolvePackageDisplayName(code string) string {
	lower := strings.ToLower(code)
	if strings.Contains(lower, "checkin") {
		return "每日签到奖励"
	}
	if strings.Contains(lower, "bonus") {
		return "平台奖励积分 (Bonus Pack)"
	}
	if strings.Contains(lower, "monthly") || strings.Contains(lower, "free") {
		return "月度免费配额"
	}
	if strings.Contains(lower, "trial") {
		return "Expert 试用权益"
	}
	if strings.Contains(lower, "extra") {
		return "额外积分加油包"
	}
	return "积分权益包"
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

	var dataArr []any
	if err := json.Unmarshal(env.Data, &dataArr); err == nil && len(dataArr) > 0 {
		packages = extractPackagesFromSlice(dataArr)
		packageSum := sumPackageCredits(packages)
		if packageSum > 0 {
			totalCredits = packageSum
		}
		return totalCredits, packages, nil
	}

	var dataMap map[string]any
	if err := json.Unmarshal(env.Data, &dataMap); err != nil {
		return 0, nil, nil
	}

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

		code := firstString(m, "packageCode", "package_code", "code", "id")
		name := firstString(m, "packageName", "package_name", "name", "title", "packageType", "type")
		if name == "" {
			name = resolvePackageDisplayName(code)
		}

		rem := firstFloat(m, "remainingCredits", "remaining_credits", "remaining", "balance", "amount", "credits", "available")
		initVal := firstFloat(m, "initialCredits", "initial_credits", "initial", "total")
		effTime := int64(firstFloat(m, "effectiveTime", "effective_time", "start_time", "startTime"))
		expTime := int64(firstFloat(m, "expireTime", "expire_time", "end_time", "endTime"))
		status := int(firstFloat(m, "status", "state"))
		if status == 0 && rem > 0 {
			status = 1
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
	return math.Round(sum*100) / 100
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
