package executor

import (
	"encoding/json"
	"strings"
)

// UsageInfo 包含 Token 统计与上游真实 Credit 计费消耗
type UsageInfo struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	Credit           float64 `json:"credit"`
}

type terminalChunkData struct {
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Credit float64 `json:"credit"`
}

// ParseTerminalUsage 解析末尾 SSE 块中的 Credit 与 Token 计数
func ParseTerminalUsage(rawChunk []byte) *UsageInfo {
	text := string(rawChunk)
	line := strings.TrimSpace(text)
	if !strings.HasPrefix(line, "data:") {
		return nil
	}

	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "[DONE]" || payload == "" {
		return nil
	}

	var parsed terminalChunkData
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil
	}

	if parsed.Usage == nil && parsed.Credit == 0 {
		return nil
	}

	info := &UsageInfo{
		Credit: parsed.Credit,
	}

	if parsed.Usage != nil {
		info.PromptTokens = parsed.Usage.PromptTokens
		info.CompletionTokens = parsed.Usage.CompletionTokens
		info.TotalTokens = parsed.Usage.TotalTokens
		if parsed.Usage.PromptTokensDetails != nil {
			info.CachedTokens = parsed.Usage.PromptTokensDetails.CachedTokens
		}
	}

	return info
}
