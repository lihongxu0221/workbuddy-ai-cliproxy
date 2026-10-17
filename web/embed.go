package web

import (
	"embed"
	"fmt"
	"strings"
	"sync"
)

// StaticFS 静态网页资源文件系统
//
//go:embed index.html css/* js/*
var StaticFS embed.FS

var (
	baseHTMLOnce  sync.Once
	baseHTMLString string
)

func getBaseHTML() string {
	baseHTMLOnce.Do(func() {
		htmlBytes, err := StaticFS.ReadFile("index.html")
		if err != nil {
			baseHTMLString = "<!doctype html><title>WorkBuddy AI</title><body>Failed to load HTML</body>"
			return
		}

		cssBytes, _ := StaticFS.ReadFile("css/dashboard.css")
		jsBytes, _ := StaticFS.ReadFile("js/app.js")

		htmlStr := string(htmlBytes)
		cssTag := "<style>\n" + string(cssBytes) + "\n</style>"
		htmlStr = strings.Replace(htmlStr, `<link rel="stylesheet" href="css/dashboard.css">`, cssTag, 1)

		jsTag := "<script>\n" + string(jsBytes) + "\n</script>"
		htmlStr = strings.Replace(htmlStr, `<script src="js/app.js"></script>`, jsTag, 1)

		baseHTMLString = htmlStr
	})
	return baseHTMLString
}

// RenderFullHTML 将 index.html, dashboard.css 和 app.js 合并成单文件 HTML，并可动态注入鉴权密钥
func RenderFullHTML(injectedKey ...string) []byte {
	htmlStr := getBaseHTML()

	var key string
	if len(injectedKey) > 0 {
		key = strings.TrimSpace(injectedKey[0])
	}

	if key != "" {
		injection := fmt.Sprintf("<script>\n  window.__CPA_MANAGEMENT_KEY__ = %q;\n</script>\n</head>", key)
		htmlStr = strings.Replace(htmlStr, "</head>", injection, 1)
	}

	return []byte(htmlStr)
}
