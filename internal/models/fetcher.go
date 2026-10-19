package models

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
)

// DynamicFetcher 负责从 WorkBuddy 上游拉取最新上架模型列表
// 严格对齐官方客户端协议：GET {base}/console/enterprises/{enterpriseId|personal}/models
// 响应含 models[] (id/name/credits/contextWindow/disabled...) 与 agents[] (name=cli 的模型白名单)
type DynamicFetcher struct {
	client *http.Client
}

// NewDynamicFetcher 实例化拉取器
func NewDynamicFetcher() *DynamicFetcher {
	return &DynamicFetcher{
		client: config.NewHTTPClient(25 * time.Second),
	}
}

// generateRequestID 生成与客户端 generateRequestId() 一致的 32 位随机十六进制串
func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// flexNum 宽容解析上游返回的数值字段 (兼容 float64 / int / string / json.Number)
func flexNum(v any) float64 {
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
		var f float64
		_, _ = fmt.Sscanf(strings.TrimSpace(x), "%g", &f)
		return f
	default:
		return 0
	}
}

// flexBool 宽容解析布尔字段 (兼容 true/"true"/1)
func flexBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "true") || strings.TrimSpace(x) == "1"
	case float64:
		return x != 0
	default:
		return false
	}
}

// deriveVendor 依据模型 ID 前缀推断厂商 (客户端响应不含 vendor 字段)
func deriveVendor(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "gemini"):
		return "Google"
	case strings.Contains(lower, "claude"):
		return "Anthropic"
	case strings.Contains(lower, "gpt"), strings.Contains(lower, "o1"), strings.Contains(lower, "o3"), strings.Contains(lower, "codex"):
		return "OpenAI"
	case strings.Contains(lower, "deepseek"):
		return "DeepSeek"
	case strings.Contains(lower, "glm"):
		return "Zhipu"
	case strings.Contains(lower, "kimi"):
		return "Moonshot"
	case strings.Contains(lower, "hunyuan"), strings.HasPrefix(lower, "hy"):
		return "Tencent"
	case strings.Contains(lower, "minimax"):
		return "MiniMax"
	case strings.Contains(lower, "grok"):
		return "xAI"
	case strings.Contains(lower, "llama"):
		return "Meta"
	case strings.Contains(lower, "qwen"):
		return "Alibaba"
	case strings.Contains(lower, "doubao"):
		return "ByteDance"
	default:
		return "WorkBuddy"
	}
}

type upstreamModelItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Credits         any    `json:"credits"`
	ContextWindow   any    `json:"contextWindow"`
	MaxInputTokens  any    `json:"maxInputTokens"`
	MaxOutputTokens any    `json:"maxOutputTokens"`
	IsDefault       any    `json:"isDefault"`
	Disabled        any    `json:"disabled"`
	DisabledReason  string `json:"disabledReason"`
	SupportsImages  any    `json:"supportsImages"`
	SupportsReasoning any  `json:"supportsReasoning"`
	// 兼容 snake_case 变体
	IDAlt             any `json:"model_id"`
	CreditsAlt        any `json:"credit_multiplier"`
	ContextWindowAlt  any `json:"context_length"`
	DisabledSnakeName any `json:"is_disabled"`
	SupportsImagesAlt any `json:"supports_images"`
	SupportsReasonAlt any `json:"supports_reasoning"`
}

type upstreamConfigResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
	// 直接返回 (无信封) 的兼容
	Models []upstreamModelItem `json:"models"`
	Agents []struct {
		Name   string   `json:"name"`
		Models []string `json:"models"`
	} `json:"agents"`
}

// FetchModelsByAccount 使用指定账号的凭据获取其可见的模型目录 (与官方客户端同源同参)
func (f *DynamicFetcher) FetchModelsByAccount(ctx context.Context, acc *auth.WorkBuddyAuth) ([]*ModelInfo, error) {
	if acc == nil {
		return nil, fmt.Errorf("account is nil")
	}

	enterpriseID := strings.TrimSpace(acc.EnterpriseID)
	if enterpriseID == "" {
		enterpriseID = "personal"
	}
	apiURL := fmt.Sprintf("%s/console/enterprises/%s/models", acc.BaseURL(), enterpriseID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Request-ID", generateRequestID())

	if acc.Realm == auth.RealmCN {
		req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
		req.Header.Set("Origin", "https://www.codebuddy.cn")
		req.Header.Set("Referer", "https://www.codebuddy.cn/")
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
		req.Header.Set("User-Agent", "WorkBuddyAI/5.8.4 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
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

	var envelope upstreamConfigResp
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode models failed: %w", err)
	}

	modelsRaw := envelope.Models

	// 腾讯网关信封格式 {code:0, data:{models, agents}}
	if len(modelsRaw) == 0 && len(envelope.Data) > 0 {
		var inner upstreamConfigResp
		if err := json.Unmarshal(envelope.Data, &inner); err == nil {
			modelsRaw = inner.Models
		}
	}

	if len(modelsRaw) == 0 {
		return nil, fmt.Errorf("upstream returned empty model catalog")
	}

	var result []*ModelInfo
	for _, item := range modelsRaw {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			if s, ok := item.IDAlt.(string); ok {
				id = strings.TrimSpace(s)
			}
		}
		if id == "" {
			continue
		}

		// 仅跳过已被官方显式标记禁用的模型，保留上游下发的所有可用模型 (含 DeepSeek、Claude、GPT 等全量目录)
		if flexBool(item.Disabled) || flexBool(item.DisabledSnakeName) {
			continue
		}

		credits := flexNum(item.Credits)
		if credits == 0 {
			credits = flexNum(item.CreditsAlt)
		}
		contextLen := flexNum(item.ContextWindow)
		if contextLen == 0 {
			contextLen = flexNum(item.ContextWindowAlt)
		}
		if contextLen == 0 {
			contextLen = flexNum(item.MaxInputTokens)
		}

		displayName := strings.TrimSpace(item.Name)
		if displayName == "" {
			displayName = id
		}

		desc := strings.TrimSpace(item.Description)
		hasImages := flexBool(item.SupportsImages) || flexBool(item.SupportsImagesAlt) || strings.Contains(id, "5v") || strings.Contains(id, "vision") || strings.Contains(id, "flash")
		hasReasoning := flexBool(item.SupportsReasoning) || flexBool(item.SupportsReasonAlt) || strings.Contains(id, "hy3") || strings.Contains(id, "fable") || strings.Contains(id, "pro")

		result = append(result, &ModelInfo{
			ID:                id,
			DisplayName:       displayName,
			Vendor:            deriveVendor(id),
			Description:       desc,
			Realm:             acc.Realm,
			CreditMultiplier:  credits,
			ContextLength:     int(contextLen),
			MaxInputTokens:    int(flexNum(item.MaxInputTokens)),
			MaxOutputTokens:   int(flexNum(item.MaxOutputTokens)),
			IsFree:            credits == 0,
			SupportsImages:    hasImages,
			SupportsTools:     true,
			SupportsReasoning: hasReasoning,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no usable models after cli-agent whitelist filter")
	}

	return result, nil
}
