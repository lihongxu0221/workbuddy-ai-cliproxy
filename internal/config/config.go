package config

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PluginConfig 插件全局可配置项
type PluginConfig struct {
	// DailyBonus 是否启用国际区每日 30 积分成长打卡 (默认 true)
	DailyBonus bool `json:"daily_bonus" yaml:"daily_bonus"`
	// CheckinAuto 是否启用国内区每日 100 积分自动签到 (默认 true)
	CheckinAuto bool `json:"checkin_auto" yaml:"checkin_auto"`
	// MaxConcurrency 单账号最大活跃并发数 (默认 2)
	MaxConcurrency int `json:"max_concurrency" yaml:"max_concurrency"`
	// CNPrefix 国内区模型路由前缀 (默认 "cn/")
	CNPrefix string `json:"cn_prefix" yaml:"cn_prefix"`
	// GlobalPrefix 国际区模型路由前缀 (默认 "cb/")
	GlobalPrefix string `json:"global_prefix" yaml:"global_prefix"`
	// SessionAffinity 是否启用软会话粘性以保护 Prompt Cache (默认 true)
	SessionAffinity bool `json:"session_affinity" yaml:"session_affinity"`
	// CacheTrace 是否开启无载荷缓存命中跟踪 (默认 false)
	CacheTrace bool `json:"cache_trace" yaml:"cache_trace"`
	// DataDir 本地持久化运行状态数据目录 (默认 ".codebuddy_data")
	DataDir string `json:"data_dir" yaml:"data_dir"`
	// ManagementKey 管理 API 访问鉴权密钥 (CPA 宿主 remote-management.secret-key)
	ManagementKey string `json:"management_key" yaml:"management_key"`
	// ProxyURL 代理服务器地址 (如 "socks5://172.19.0.1:10808" 或 "http://127.0.0.1:7890")
	ProxyURL string `json:"proxy_url" yaml:"proxy_url"`
	// SelfURL CPA 宿主基地址 (用于触发宿主重载等回调)
	SelfURL string `json:"self_url" yaml:"self_url"`
}

var (
	globalConfig = DefaultConfig()
	configMu     sync.RWMutex
)

// DefaultConfig 提供生产级安全默认值
func DefaultConfig() *PluginConfig {
	dataDir := os.Getenv("WORKBUDDY_DATA_DIR")
	if dataDir == "" {
		dataDir = ".codebuddy_data"
	}

	mgmtKey := detectInitialManagementKey()
	proxyURL := detectInitialProxyURL()

	return &PluginConfig{
		DailyBonus:      true,
		CheckinAuto:     true,
		MaxConcurrency:  2,
		CNPrefix:        "cn/",
		GlobalPrefix:    "cb/",
		SessionAffinity: true,
		CacheTrace:      os.Getenv("WB_CACHE_TRACE") == "1",
		DataDir:         dataDir,
		ManagementKey:   mgmtKey,
		ProxyURL:        proxyURL,
		SelfURL:         os.Getenv("WB_SELF_URL"),
	}
}

// detectInitialManagementKey 优先从环境变量或 CPA 宿主 config.yaml 自动探测未哈希的明文密钥
func detectInitialManagementKey() string {
	// 1. 环境变量
	envs := []string{"WB_MANAGEMENT_KEY", "CPA_MANAGEMENT_KEY", "MANAGEMENT_KEY"}
	for _, env := range envs {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}

	// 2. 探测当前及父级目录下的 config.yaml
	candidateFiles := []string{
		"config.yaml",
		filepath.Join("..", "config.yaml"),
		filepath.Join("..", "..", "config.yaml"),
	}

	for _, f := range candidateFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "management_key:") || strings.HasPrefix(line, "management-key:") {
				val := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "management_key:"), "management-key:"))
				val = strings.Trim(val, `"' `)
				if val != "" && !strings.HasPrefix(val, "$2a$") {
					return val
				}
			}
			if strings.HasPrefix(line, "secret-key:") || strings.HasPrefix(line, "secret_key:") {
				val := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "secret-key:"), "secret_key:"))
				val = strings.Trim(val, `"' `)
				if val != "" && !strings.HasPrefix(val, "$2a$") {
					return val
				}
			}
		}
	}

	return ""
}

// detectInitialProxyURL 自动探测环境或 CPA 宿主全局配置中的 proxy-url (如 socks5://172.19.0.1:10808)
func detectInitialProxyURL() string {
	// 1. 优先读取系统代理环境变量
	proxyEnvs := []string{"WB_PROXY_URL", "ALL_PROXY", "all_proxy", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"}
	for _, env := range proxyEnvs {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}

	// 2. 探测 CPA 宿主 config.yaml 中的 proxy-url
	candidateFiles := []string{
		"config.yaml",
		filepath.Join("..", "config.yaml"),
		filepath.Join("..", "..", "config.yaml"),
	}

	for _, f := range candidateFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "proxy-url:") || strings.HasPrefix(line, "proxy_url:") {
				val := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "proxy-url:"), "proxy_url:"))
				val = strings.Trim(val, `"' `)
				if val != "" {
					return val
				}
			}
		}
	}

	return ""
}

// Get 返回当前插件生效的配置快照
func Get() *PluginConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	cfgCopy := *globalConfig
	return &cfgCopy
}

// GetManagementKey 获取当前探测或配置的管理密钥
func GetManagementKey() string {
	cfg := Get()
	if cfg.ManagementKey != "" {
		return cfg.ManagementKey
	}
	return detectInitialManagementKey()
}

// GetProxyURL 获取当前生效的代理地址 (优先用户配置，兜底宿主 proxy-url)
func GetProxyURL() string {
	cfg := Get()
	if cfg.ProxyURL != "" {
		return cfg.ProxyURL
	}
	return detectInitialProxyURL()
}

// NewHTTPClient 创建统一的具有代理支持与 TLS1.2+ 的 HTTP 客户端
func NewHTTPClient(timeout time.Duration, proxyOverride ...string) *http.Client {
	proxyStr := GetProxyURL()
	if len(proxyOverride) > 0 && strings.TrimSpace(proxyOverride[0]) != "" {
		proxyStr = strings.TrimSpace(proxyOverride[0])
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	if proxyStr != "" {
		if parsed, err := url.Parse(proxyStr); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// Update 更新并合并运行时配置
func Update(newCfg *PluginConfig) {
	configMu.Lock()
	defer configMu.Unlock()
	if newCfg != nil {
		globalConfig = newCfg
	}
}

// LoadFromMap 从 CPA 宿主传入的通用 map 解析配置
func LoadFromMap(raw map[string]any) error {
	rawBytes, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(rawBytes, cfg); err != nil {
		return err
	}

	// 兼容多种 key 命名变体
	keyAliases := []string{
		"management_key", "management-key", "managementKey",
		"secret_key", "secret-key", "secretKey", "key",
	}
	for _, alias := range keyAliases {
		if v, ok := raw[alias]; ok {
			if strVal, isStr := v.(string); isStr && strings.TrimSpace(strVal) != "" {
				cfg.ManagementKey = strings.TrimSpace(strVal)
				break
			}
		}
	}

	// 兼容多种 proxy 命名变体
	proxyAliases := []string{"proxy_url", "proxy-url", "proxyUrl", "proxy"}
	for _, alias := range proxyAliases {
		if v, ok := raw[alias]; ok {
			if strVal, isStr := v.(string); isStr && strings.TrimSpace(strVal) != "" {
				cfg.ProxyURL = strings.TrimSpace(strVal)
				break
			}
		}
	}

	Update(cfg)
	return nil
}

// EnsureDataDir 保证运行时数据目录就绪
func (c *PluginConfig) EnsureDataDir() error {
	if c.DataDir == "" {
		c.DataDir = ".codebuddy_data"
	}
	return os.MkdirAll(c.DataDir, 0755)
}

// ResolvePath 解析相对于数据目录的文件路径
func (c *PluginConfig) ResolvePath(elem ...string) string {
	parts := append([]string{c.DataDir}, elem...)
	return filepath.Join(parts...)
}
