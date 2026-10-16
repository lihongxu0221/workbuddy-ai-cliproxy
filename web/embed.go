package web

import "embed"

// StaticFS 静态网页资源文件系统
//
//go:embed index.html css/* js/*
var StaticFS embed.FS
