package config

import (
	"path/filepath"
	"testing"
)

func TestResolvePathsWithHostAuthDir(t *testing.T) {
	fakeHostAuthDir, _ := filepath.Abs(filepath.Clean("/root/.cli-proxy-api"))
	fakeConfigDir, _ := filepath.Abs(filepath.Clean("/etc/cpa"))

	hostCtx := cpaHostContext{
		configDir:     fakeConfigDir,
		authDir:       fakeHostAuthDir,
		managementKey: "test-secret-key",
		proxyURL:      "socks5://172.19.0.1:10808",
	}

	// 1. 当插件未显式配置 auth_dir 与 data_dir 时，自动使用 CPA 挂载卷下的子目录
	cfg := &PluginConfig{}
	resolvePaths(cfg, hostCtx)

	expectedAuth := filepath.Join(fakeHostAuthDir, "workbuddy")
	expectedData := filepath.Join(fakeHostAuthDir, ".codebuddy_data")

	if cfg.ResolvedAuthDir != expectedAuth {
		t.Errorf("ResolvedAuthDir = %s, want %s", cfg.ResolvedAuthDir, expectedAuth)
	}
	if cfg.ResolvedDataDir != expectedData {
		t.Errorf("ResolvedDataDir = %s, want %s", cfg.ResolvedDataDir, expectedData)
	}

	// 2. 当配置相对路径时，优先相对宿主持久卷解析
	cfgCustom := &PluginConfig{
		AuthDir: "my_auths",
		DataDir: "my_data",
	}
	resolvePaths(cfgCustom, hostCtx)

	expectedCustomAuth := filepath.Join(fakeHostAuthDir, "my_auths")
	expectedCustomData := filepath.Join(fakeHostAuthDir, "my_data")

	if cfgCustom.ResolvedAuthDir != expectedCustomAuth {
		t.Errorf("Custom ResolvedAuthDir = %s, want %s", cfgCustom.ResolvedAuthDir, expectedCustomAuth)
	}
	if cfgCustom.ResolvedDataDir != expectedCustomData {
		t.Errorf("Custom ResolvedDataDir = %s, want %s", cfgCustom.ResolvedDataDir, expectedCustomData)
	}
}

func TestGetAuthDirAndDataDir(t *testing.T) {
	authDir := GetAuthDir()
	if authDir == "" {
		t.Errorf("GetAuthDir() should not be empty")
	}

	dataDir := GetDataDir()
	if dataDir == "" {
		t.Errorf("GetDataDir() should not be empty")
	}
}
