package web

import (
	"embed"
	"strings"
	"sync"
)

// StaticFS 静态网页资源文件系统
//
//go:embed index.html css/* js/*
var StaticFS embed.FS

var (
	fullHTMLOnce  sync.Once
	fullHTMLBytes []byte
)

// RenderFullHTML 将 index.html, dashboard.css 和 app.js 合并成无外部依赖的单文件 HTML
func RenderFullHTML() []byte {
	fullHTMLOnce.Do(func() {
		htmlBytes, err := StaticFS.ReadFile("index.html")
		if err != nil {
			fullHTMLBytes = []byte("<!doctype html><title>WorkBuddy AI</title><body>Failed to load HTML</body>")
			return
		}

		cssBytes, _ := StaticFS.ReadFile("css/dashboard.css")
		jsBytes, _ := StaticFS.ReadFile("js/app.js")

		htmlStr := string(htmlBytes)
		cssTag := "<style>\n" + string(cssBytes) + "\n</style>"
		htmlStr = strings.Replace(htmlStr, `<link rel="stylesheet" href="css/dashboard.css">`, cssTag, 1)

		jsTag := "<script>\n" + string(jsBytes) + "\n</script>"
		htmlStr = strings.Replace(htmlStr, `<script src="js/app.js"></script>`, jsTag, 1)

		fullHTMLBytes = []byte(htmlStr)
	})
	return fullHTMLBytes
}
