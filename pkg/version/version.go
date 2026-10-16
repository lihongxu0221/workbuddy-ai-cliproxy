package version

import (
	"fmt"
	"runtime"
)

var (
	// Version 插件发布版本号
	Version = "1.0.0"
	// GitCommit 构建时的 Git 提交哈希
	GitCommit = "HEAD"
	// BuildDate 构建日期
	BuildDate = "2026-10-16"
	// PluginID CLIProxyAPI 内部插件唯一标识
	PluginID = "workbuddy"
	// PluginName 人类可读名称
	PluginName = "WorkBuddy AI Proxy Plugin"
	// Author 作者与维护团队
	Author = "workbuddy-ai-team"
)

// Info 返回格式化的版本详细信息
func Info() string {
	return fmt.Sprintf("%s v%s (commit: %s, built: %s, %s/%s)",
		PluginName, Version, GitCommit, BuildDate, runtime.GOOS, runtime.GOARCH)
}
