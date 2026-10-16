package translator

import (
	"encoding/json"
	"fmt"
	"strings"
	"workbuddy-ai-cliproxy/internal/models"
)

// GeminiContentPart 单个内容块
type GeminiContentPart struct {
	Text string `json:"text,omitempty"`
}

// GeminiContent 单条上下文
type GeminiContent struct {
	Role  string              `json:"role"`
	Parts []GeminiContentPart `json:"parts"`
}

// GeminiGenerateRequest Gemini 原生生成请求结构
type GeminiGenerateRequest struct {
	Contents          []GeminiContent `json:"contents"`
	SystemInstruction *struct {
		Parts []GeminiContentPart `json:"parts"`
	} `json:"systemInstruction,omitempty"`
}

// TranslateGeminiToOpenAI 将 Google Gemini 请求转译为 WorkBuddy (OpenAI-compatible) 格式
func TranslateGeminiToOpenAI(rawModel string, rawJSON []byte, stream bool) (targetModel string, translatedPayload []byte, err error) {
	var req GeminiGenerateRequest
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return "", nil, fmt.Errorf("unmarshal gemini request failed: %w", err)
	}

	parsedModel := models.ParsePrefix(rawModel)
	targetModel = parsedModel.CleanModel

	var openaiMessages []map[string]any

	// 1. 处理 SystemInstruction
	if req.SystemInstruction != nil {
		var sb strings.Builder
		for _, p := range req.SystemInstruction.Parts {
			sb.WriteString(p.Text)
		}
		if sb.Len() > 0 {
			openaiMessages = append(openaiMessages, map[string]any{
				"role":    "system",
				"content": sb.String(),
			})
		}
	}

	// 2. 处理 Contents
	for _, c := range req.Contents {
		role := "user"
		if c.Role == "model" {
			role = "assistant"
		}
		var sb strings.Builder
		for _, p := range c.Parts {
			sb.WriteString(p.Text)
		}
		openaiMessages = append(openaiMessages, map[string]any{
			"role":    role,
			"content": sb.String(),
		})
	}

	mapped := map[string]any{
		"model":    targetModel,
		"messages": openaiMessages,
		"stream":   stream,
	}

	outBytes, err := json.Marshal(mapped)
	return targetModel, outBytes, err
}
