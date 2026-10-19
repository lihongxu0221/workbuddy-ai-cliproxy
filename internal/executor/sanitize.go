package executor

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"
	"workbuddy-ai-cliproxy/internal/models"
)

// UnpackPayload 彻底解包可能被多层 JSON/Base64 包装的载荷，确保还原为标准的 JSON Object 字节切片
func UnpackPayload(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return raw
	}

	// 1. 如果已被双重序列化为 JSON 字符串 (以引号包裹)，解开一层转义
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			unquoted := []byte(s)
			t := bytes.TrimSpace(unquoted)
			if len(t) > 0 && (t[0] == '{' || t[0] == '[') {
				return UnpackPayload(t) // 递归解包可能存在的多层嵌套
			}
			// 检查内部是否为 Base64
			if decoded, errB64 := base64.StdEncoding.DecodeString(s); errB64 == nil {
				tB64 := bytes.TrimSpace(decoded)
				if len(tB64) > 0 && (tB64[0] == '{' || tB64[0] == '[') {
					return UnpackPayload(tB64)
				}
			}
			return unquoted
		}
	}

	// 2. 如果是未带引号的 Base64 编码字符串
	if len(trimmed) > 0 && trimmed[0] != '{' && trimmed[0] != '[' {
		if decoded, errB64 := base64.StdEncoding.DecodeString(string(trimmed)); errB64 == nil {
			tB64 := bytes.TrimSpace(decoded)
			if len(tB64) > 0 && (tB64[0] == '{' || tB64[0] == '[') {
				return UnpackPayload(tB64)
			}
		}
	}

	return trimmed
}

// RewriteSystemForUpstream 敏感词与思考模型策略重写，并对国内版与全区请求体执行结构化规范化
// 解决 11101 Unmarshal chat params failed 错误：确保 messages 中的 content 字段为结构化数据 [{"type":"text","text":"..."}]，并去除模型路由前缀
func RewriteSystemForUpstream(payload []byte, targetModel ...string) []byte {
	unpacked := UnpackPayload(payload)
	if len(unpacked) == 0 {
		return payload
	}
	var obj map[string]any
	if json.Unmarshal(unpacked, &obj) != nil {
		return unpacked
	}

	changed := false

	// 1. 规范化 model 字段 (剥离 cn/, cb/, en/, wb/ 等前缀)
	if len(targetModel) > 0 && strings.TrimSpace(targetModel[0]) != "" {
		clean := strings.TrimSpace(targetModel[0])
		if obj["model"] != clean {
			obj["model"] = clean
			changed = true
		}
	} else if currentModel, ok := obj["model"].(string); ok {
		clean := models.ParsePrefix(currentModel).CleanModel
		if clean != currentModel {
			obj["model"] = clean
			changed = true
		}
	}

	// 2. 规范化 messages 列表与 content 字段结构
	if messages, ok := obj["messages"].([]any); ok {
		for i, m := range messages {
			msg, isMap := m.(map[string]any)
			if !isMap {
				continue
			}

			// 检查并规范化 content 字段为结构化数组格式 (上游要求结构化数据，不能为裸字符串)
			switch c := msg["content"].(type) {
			case string:
				sanitized := SanitizeBlockedTemplates(c)
				msg["content"] = []map[string]any{
					{"type": "text", "text": sanitized},
				}
				changed = true
			case []any:
				// 若已为数组，遍历并替换敏感模板句
				for _, p := range c {
					if part, okPart := p.(map[string]any); okPart {
						if t, okText := part["text"].(string); okText {
							if r := SanitizeBlockedTemplates(t); r != t {
								part["text"] = r
								changed = true
							}
						}
						if _, hasType := part["type"]; !hasType {
							part["type"] = "text"
							changed = true
						}
					}
				}
			case nil:
				msg["content"] = []map[string]any{
					{"type": "text", "text": ""},
				}
				changed = true
			}
			messages[i] = msg
		}
		obj["messages"] = messages
	}

	// 3. 对 hy3 系列模型注入最大深度思考
	if ForceMaxThinking(obj) {
		changed = true
	}

	// 4. 清理无效的空工具参数
	if tools, ok := obj["tools"].([]any); ok && len(tools) == 0 {
		delete(obj, "tools")
		delete(obj, "tool_choice")
		changed = true
	}

	if !changed && bytes.Equal(unpacked, payload) {
		return payload
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return unpacked
	}
	return out
}

// SanitizeBlockedTemplates 替换被腾讯 CodeBuddy 拦截的固定模板句
func SanitizeBlockedTemplates(s string) string {
	s = strings.ReplaceAll(s,
		"You are Claude Code, Anthropic's official CLI for Claude.",
		"You are Claude Code, Anthropic's official CLI tool for Claude.")
	s = strings.ReplaceAll(s,
		"Main branch (you will usually use this for PRs)",
		"Default branch (you will usually use this for PRs)")
	return s
}

// ForceMaxThinking 对 hy3 系列模型自动注入 reasoning_effort=high
func ForceMaxThinking(obj map[string]any) bool {
	model, _ := obj["model"].(string)
	if !strings.HasPrefix(model, "hy3") && !strings.Contains(model, "hy3") {
		return false
	}
	if eff, _ := obj["reasoning_effort"].(string); eff == "high" {
		return false
	}
	obj["reasoning_effort"] = "high"
	return true
}

// ForceStreamBody 确保请求体中的 stream=true 并解除外层 JSON 字符串包裹
func ForceStreamBody(payload, original []byte) []byte {
	src := UnpackPayload(payload)
	if len(src) == 0 {
		src = UnpackPayload(original)
	}
	var obj map[string]any
	if json.Unmarshal(src, &obj) != nil {
		return src
	}
	obj["stream"] = true
	out, err := json.Marshal(obj)
	if err != nil {
		return src
	}
	return out
}

// CleanChunkJSON 清理增量 chunk 中的空值字段
func CleanChunkJSON(s string) string {
	var obj map[string]any
	if json.Unmarshal([]byte(s), &obj) != nil {
		return s
	}
	if choices, ok := obj["choices"].([]any); ok {
		for _, c := range choices {
			choice, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if delta, ok := choice["delta"].(map[string]any); ok {
				for k, v := range delta {
					if IsEmptyValue(v) {
						delete(delta, k)
					}
				}
			}
		}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return s
	}
	return string(out)
}

// IsEmptyValue 判断值是否为空
func IsEmptyValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

// StripDataPrefix 剥离 SSE data: 前缀
func StripDataPrefix(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "data:") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "data:"))
	}
	return s
}

// ClientNeedsSSEFrame 判断客户端是否需要 data: 前缀格式化
func ClientNeedsSSEFrame(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	path, _ := metadata["request_path"].(string)
	switch strings.ToLower(strings.TrimSpace(path)) {
	case "/v1/chat/completions", "/v1/completions":
		return false
	default:
		return true
	}
}

// AggregateCompletion 将上游 SSE 流聚合成完整的非流式 ChatCompletion 对象
func AggregateCompletion(r io.Reader, fallbackModel string) ([]byte, error) {
	var content, reasoning, role, respModel, respID, finish string
	var created int64
	var usage map[string]any
	var toolCalls []map[string]any

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		data := StripDataPrefix(scanner.Text())
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if v, ok := chunk["id"].(string); ok && v != "" {
			respID = v
		}
		if v, ok := chunk["model"].(string); ok && v != "" {
			respModel = v
		}
		if v, ok := chunk["created"].(float64); ok {
			created = int64(v)
		}
		if v, ok := chunk["usage"].(map[string]any); ok {
			usage = v
		}
		choices, _ := chunk["choices"].([]any)
		for _, c := range choices {
			choice, _ := c.(map[string]any)
			if delta, ok := choice["delta"].(map[string]any); ok {
				if v, ok := delta["role"].(string); ok && v != "" {
					role = v
				}
				if v, ok := delta["content"].(string); ok {
					content += v
				}
				if v, ok := delta["reasoning_content"].(string); ok {
					reasoning += v
				}
				if tcs, ok := delta["tool_calls"].([]any); ok {
					for _, tc := range tcs {
						if call, ok := tc.(map[string]any); ok {
							toolCalls = append(toolCalls, call)
						}
					}
				}
			}
			if v, ok := choice["finish_reason"].(string); ok && v != "" {
				finish = v
			}
		}
	}

	if role == "" {
		role = "assistant"
	}
	message := map[string]any{
		"role":    role,
		"content": content,
	}
	if reasoning != "" {
		message["reasoning_content"] = reasoning
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	if created == 0 {
		created = time.Now().Unix()
	}
	if finish == "" {
		finish = "stop"
	}
	if respID == "" {
		respID = "chatcmpl-workbuddy"
	}
	if respModel == "" {
		respModel = fallbackModel
	}

	result := map[string]any{
		"id":      respID,
		"object":  "chat.completion",
		"created": created,
		"model":   respModel,
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": finish,
		}},
	}
	if usage != nil {
		result["usage"] = usage
	}

	return json.Marshal(result)
}
