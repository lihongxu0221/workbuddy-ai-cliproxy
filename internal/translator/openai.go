package translator

import (
	"encoding/json"
	"workbuddy-ai-cliproxy/internal/models"
)

// OpenAIChatRequest 标准 OpenAI 对话请求结构体
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []json.RawMessage `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Tools       []ToolDef       `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
}

// TranslateOpenAIRequest 将 OpenAI 请求规范化并剥离模型前缀
func TranslateOpenAIRequest(rawJSON []byte) (targetModel string, parsedReq OpenAIChatRequest, translatedPayload []byte, err error) {
	if err = json.Unmarshal(rawJSON, &parsedReq); err != nil {
		return "", parsedReq, nil, err
	}

	parsedModel := models.ParsePrefix(parsedReq.Model)
	targetModel = parsedModel.CleanModel

	// 将 model 字段改写为上游识别的原生模型名
	var rawMap map[string]any
	if err = json.Unmarshal(rawJSON, &rawMap); err == nil {
		rawMap["model"] = targetModel
		translatedPayload, _ = json.Marshal(rawMap)
	} else {
		parsedReq.Model = targetModel
		translatedPayload, _ = json.Marshal(parsedReq)
	}

	return targetModel, parsedReq, translatedPayload, nil
}
