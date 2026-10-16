package translator

import (
	"encoding/json"
)

// FunctionDef 标准函数定义
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolDef 标准工具定义
type ToolDef struct {
	Type     string      `json:"type"` // 固定为 "function"
	Function FunctionDef `json:"function"`
}

// ToolCallInstance 工具调用实例
type ToolCallInstance struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// NormalizeTools 将任意格式工具清单标准化为统一数组
func NormalizeTools(raw any) []ToolDef {
	if raw == nil {
		return nil
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var tools []ToolDef
	if err := json.Unmarshal(bytes, &tools); err == nil {
		return tools
	}
	return nil
}
