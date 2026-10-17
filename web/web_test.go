package web

import (
	"strings"
	"testing"
)

func TestRenderFullHTML(t *testing.T) {
	htmlBytes := RenderFullHTML()
	if len(htmlBytes) == 0 {
		t.Fatalf("expected non-empty full HTML")
	}

	htmlStr := string(htmlBytes)

	// 必须包含核心 DOM 元素
	if !strings.Contains(htmlStr, "WorkBuddy AI 控制台") {
		t.Errorf("expected HTML to contain title")
	}
	if !strings.Contains(htmlStr, "modal-login") {
		t.Errorf("expected HTML to contain login modal")
	}

	// 必须内嵌 style 和 script，不能保留外部引用
	if strings.Contains(htmlStr, `<link rel="stylesheet" href="css/dashboard.css">`) {
		t.Errorf("HTML should not contain external link tag for css")
	}
	if strings.Contains(htmlStr, `<script src="js/app.js"></script>`) {
		t.Errorf("HTML should not contain external script tag for js")
	}

	if !strings.Contains(htmlStr, "<style>") || !strings.Contains(htmlStr, "</style>") {
		t.Errorf("HTML must contain inline <style>")
	}
	if !strings.Contains(htmlStr, "<script>") || !strings.Contains(htmlStr, "</script>") {
		t.Errorf("HTML must contain inline <script>")
	}

	// 验证包含管理路由基地址
	if !strings.Contains(htmlStr, "/v0/management/workbuddyai") {
		t.Errorf("expected inline script to contain /v0/management/workbuddyai route base")
	}

	// 验证动态注入 key
	htmlWithKey := string(RenderFullHTML("test-secret-key-123"))
	if !strings.Contains(htmlWithKey, `window.__CPA_MANAGEMENT_KEY__ = "test-secret-key-123"`) {
		t.Errorf("expected HTML to contain injected key script")
	}
}
