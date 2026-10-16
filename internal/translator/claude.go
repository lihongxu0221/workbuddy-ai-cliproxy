package translator

import (
	"encoding/json"
	"fmt"
	"strings"
	"workbuddy-ai-cliproxy/internal/models"
)

// AnthropicMessage 单条 Claude 消息
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string 或 []ContentBlock
}

// AnthropicRequest 标准 Claude Messages 请求结构体
type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []AnthropicMessage `json:"messages"`
	System    any                `json:"system,omitempty"` // string 或 []SystemBlock
	Stream    bool               `json:"stream,omitempty"`
	MaxTokens int                `json:"max_tokens,omitempty"`
	Thinking  *struct {
		Type         string `json:"type"`
		BudgetTokens int    `json:"budget_tokens,omitempty"`
	} `json:"thinking,omitempty"`
}

// TranslateClaudeToOpenAI 将 Anthropic Messages 协议转译为 WorkBuddy (OpenAI-compatible) 格式
func TranslateClaudeToOpenAI(rawJSON []byte) (targetModel string, translatedPayload []byte, err error) {
	var req AnthropicRequest
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return "", nil, fmt.Errorf("unmarshal anthropic request failed: %w", err)
	}

	parsedModel := models.ParsePrefix(req.Model)
	targetModel = parsedModel.CleanModel

	var openaiMessages []map[string]any

	// 1. 处理 System 提示词
	if req.System != nil {
		sysText := extractSystemText(req.System)
		if sysText != "" {
			openaiMessages = append(openaiMessages, map[string]any{
				"role":    "system",
				"content": sysText,
			})
		}
	}

	// 2. 处理各轮 Message
	for _, msg := range req.Messages {
		role := msg.Role
		contentStr := extractContentText(msg.Content)
		openaiMessages = append(openaiMessages, map[string]any{
			"role":    role,
			"content": contentStr,
		})
	}

	mapped := map[string]any{
		"model":    targetModel,
		"messages": openaiMessages,
		"stream":   req.Stream,
	}

	if req.MaxTokens > 0 {
		mapped["max_tokens"] = req.MaxTokens
	}

	outBytes, err := json.Marshal(mapped)
	return targetModel, outBytes, err
}

func extractSystemText(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
					sb.WriteString("\n")
				}
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return ""
}

func extractContentText(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()
	}
	return ""
}
