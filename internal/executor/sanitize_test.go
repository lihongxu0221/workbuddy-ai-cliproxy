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
	sysContent := sysMsg["content"].(string)

	if strings.Contains(sysContent, "CLI for Claude") {
		t.Errorf("expected system message to be rewritten")
	}
	if !strings.Contains(sysContent, "CLI tool for Claude") {
		t.Errorf("expected 'CLI tool' in rewritten content")
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
