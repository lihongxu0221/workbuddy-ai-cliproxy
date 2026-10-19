package executor

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeBlockedTemplates(t *testing.T) {
	orig := "Hello. You are Claude Code, Anthropic's official CLI for Claude. Target: Main branch (you will usually use this for PRs)."
	cleaned := SanitizeBlockedTemplates(orig)

	if strings.Contains(cleaned, "CLI for Claude") {
		t.Errorf("expected 'CLI for Claude' to be sanitized")
	}
	if !strings.Contains(cleaned, "CLI tool for Claude") {
		t.Errorf("expected 'CLI tool for Claude' in output")
	}
	if strings.Contains(cleaned, "Main branch (you will usually use this for PRs)") {
		t.Errorf("expected 'Main branch' phrase to be sanitized")
	}
	if !strings.Contains(cleaned, "Default branch (you will usually use this for PRs)") {
		t.Errorf("expected 'Default branch' phrase in output")
	}
}

func TestForceMaxThinking(t *testing.T) {
	obj1 := map[string]any{
		"model": "hy3-preview-agent",
	}
	if !ForceMaxThinking(obj1) {
		t.Errorf("expected ForceMaxThinking to return true for hy3 model")
	}
	if obj1["reasoning_effort"] != "high" {
		t.Errorf("expected reasoning_effort = high, got %v", obj1["reasoning_effort"])
	}

	obj2 := map[string]any{
		"model": "gpt-6-astra",
	}
	if ForceMaxThinking(obj2) {
		t.Errorf("expected ForceMaxThinking to return false for non-hy3 model")
	}
	if _, ok := obj2["reasoning_effort"]; ok {
		t.Errorf("did not expect reasoning_effort to be set for gpt-6-astra")
	}
}

func TestRewriteSystemForUpstream(t *testing.T) {
	reqJSON := `{
		"model": "hy3",
		"messages": [
			{
				"role": "system",
				"content": "You are Claude Code, Anthropic's official CLI for Claude."
			},
			{
				"role": "user",
				"content": "hello"
			}
		],
		"stream": false
	}`

	rewritten := RewriteSystemForUpstream([]byte(reqJSON))
	var parsed map[string]any
	if err := json.Unmarshal(rewritten, &parsed); err != nil {
		t.Fatalf("unmarshal rewritten JSON failed: %v", err)
	}

	if parsed["reasoning_effort"] != "high" {
		t.Errorf("expected reasoning_effort = high for hy3, got %v", parsed["reasoning_effort"])
	}

	messages := parsed["messages"].([]any)
	sysMsg := messages[0].(map[string]any)
	contentParts := sysMsg["content"].([]any)
	part0 := contentParts[0].(map[string]any)
	sysContent := part0["text"].(string)

	if strings.Contains(sysContent, "CLI for Claude") {
		t.Errorf("expected system message to be rewritten")
	}
	if !strings.Contains(sysContent, "CLI tool for Claude") {
		t.Errorf("expected 'CLI tool' in rewritten content")
	}

	// 验证剥离 cn/ 前缀并将 string 转为结构化 content 数组
	reqWithPrefix := `{"model":"cn/deepseek-v4-pro","messages":[{"role":"user","content":"hi"}]}`
	rewrittenPrefix := RewriteSystemForUpstream([]byte(reqWithPrefix))
	var parsedPrefix map[string]any
	_ = json.Unmarshal(rewrittenPrefix, &parsedPrefix)
	if parsedPrefix["model"] != "deepseek-v4-pro" {
		t.Errorf("expected model = deepseek-v4-pro, got %v", parsedPrefix["model"])
	}
	userParts := parsedPrefix["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if userParts[0].(map[string]any)["text"] != "hi" {
		t.Errorf("expected text = hi, got %v", userParts[0])
	}
}

func TestForceStreamBody(t *testing.T) {
	reqJSON := `{"model":"deepseek-v4","stream":false}`
	forced := ForceStreamBody([]byte(reqJSON), nil)

	var parsed map[string]any
	_ = json.Unmarshal(forced, &parsed)

	if parsed["stream"] != true {
		t.Errorf("expected stream to be true, got %v", parsed["stream"])
	}
}

func TestCleanChunkJSON(t *testing.T) {
	chunkWithEmpty := `{"id":"1","choices":[{"delta":{"content":"hi","function_call":null,"tool_calls":[]}}]}`
	cleaned := CleanChunkJSON(chunkWithEmpty)

	if strings.Contains(cleaned, `"function_call"`) {
		t.Errorf("expected null function_call to be removed: %s", cleaned)
	}
	if strings.Contains(cleaned, `"tool_calls"`) {
		t.Errorf("expected empty tool_calls to be removed: %s", cleaned)
	}
	if !strings.Contains(cleaned, `"content":"hi"`) {
		t.Errorf("expected content to be preserved: %s", cleaned)
	}
}

func TestUnpackPayload(t *testing.T) {
	// 1. 原始 JSON Object
	rawObj := []byte(`{"model":"deepseek-v4-pro","stream":false}`)
	unpacked1 := UnpackPayload(rawObj)
	if string(unpacked1) != string(rawObj) {
		t.Errorf("expected raw obj preserved, got %s", string(unpacked1))
	}

	// 2. 双重序列化为 JSON 字符串 (以引号包裹) -> 解开后必须为真实对象
	doubleEncoded := []byte(`"{\"model\":\"deepseek-v4-pro\",\"stream\":false}"`)
	unpacked2 := UnpackPayload(doubleEncoded)
	if unpacked2[0] != '{' || !strings.Contains(string(unpacked2), `"model":"deepseek-v4-pro"`) {
		t.Errorf("failed to unpack double-encoded JSON string: %s", string(unpacked2))
	}

	// 验证 ForceStreamBody 和 RewriteSystemForUpstream 能够正常处理双重序列化字符串并输出标准对象
	forced := ForceStreamBody(doubleEncoded, nil)
	if forced[0] != '{' {
		t.Errorf("ForceStreamBody did not unpack string to JSON object: %s", string(forced))
	}
	rewritten := RewriteSystemForUpstream(doubleEncoded, "deepseek-v4-pro")
	if rewritten[0] != '{' {
		t.Errorf("RewriteSystemForUpstream did not unpack string to JSON object: %s", string(rewritten))
	}

	// 3. Base64 编码字符串
	b64String := []byte(`"eyJtb2RlbCI6ImRlZXBzZWVrLXY0LXBybyIsInN0cmVhbSI6ZmFsc2V9"`)
	unpacked3 := UnpackPayload(b64String)
	if unpacked3[0] != '{' {
		t.Errorf("failed to unpack base64 payload: %s", string(unpacked3))
	}
}
