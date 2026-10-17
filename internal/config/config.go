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
	// DataDir 本地持久化运行状态数据目录 (若未配则优先存于 CPA auth-dir 下的 .codebuddy_data)
	DataDir string `json:"data_dir" yaml:"data_dir"`
	// AuthDir 账号凭证存储目录 (若未配则优先存于 CPA auth-dir 下的 workbuddy 子目录)
	AuthDir string `json:"auth_dir" yaml:"auth_dir"`
	// ManagementKey 管理 API 访问鉴权密钥 (CPA 宿主 remote-management.secret-key)
	ManagementKey string `json:"management_key" yaml:"management_key"`
	// ProxyURL 代理服务器地址 (如 "socks5://172.19.0.1:10808" 或 "http://127.0.0.1:7890")
	ProxyURL string `json:"proxy_url" yaml:"proxy_url"`
	// SelfURL CPA 宿主基地址 (用于触发宿主重载等回调)
	SelfURL string `json:"self_url" yaml:"self_url"`

	// 运行时解析的绝对路径 (不依赖进程当前工作目录，优先绑定宿主持久卷)
	ResolvedAuthDir string `json:"-"`
	ResolvedDataDir string `json:"-"`
	CPAConfigDir    string `json:"-"`
	CPAAuthDir      string `json:"-"`
}

var (
	globalConfig = DefaultConfig()
	configMu     sync.RWMutex
)

// DefaultConfig 提供生产级安全默认值
func DefaultConfig() *PluginConfig {
	hostCtx := detectCPAHostContext()

	cfg := &PluginConfig{
		DailyBonus:      true,
		CheckinAuto:     true,
		MaxConcurrency:  2,
		CNPrefix:        "cn/",
		GlobalPrefix:    "cb/",
		SessionAffinity: true,
		CacheTrace:      os.Getenv("WB_CACHE_TRACE") == "1",
		DataDir:         os.Getenv("WORKBUDDY_DATA_DIR"),
		AuthDir:         os.Getenv("WORKBUDDY_AUTH_DIR"),
		ManagementKey:   hostCtx.managementKey,
		ProxyURL:        hostCtx.proxyURL,
		SelfURL:         os.Getenv("WB_SELF_URL"),
		CPAConfigDir:    hostCtx.configDir,
		CPAAuthDir:      hostCtx.authDir,
	}

	resolvePaths(cfg, hostCtx)
	return cfg
}

type cpaHostContext struct {
	configDir     string
	authDir       string
	managementKey string
	proxyURL      string
}

// detectCPAHostContext 探测 CPA 宿主的 config.yaml 文件及其配置上下文 (不依赖当前工作目录)
func detectCPAHostContext() cpaHostContext {
	ctx := cpaHostContext{}

	candidateFiles := []string{
		"config.yaml",
		filepath.Join("..", "config.yaml"),
		filepath.Join("..", "..", "config.yaml"),
	}
	if envConfig := os.Getenv("CPA_CONFIG_FILE"); envConfig != "" {
		candidateFiles = append([]string{envConfig}, candidateFiles...)
	}

	for _, f := range candidateFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		absPath, errAbs := filepath.Abs(f)
		if errAbs == nil {
			ctx.configDir = filepath.Dir(absPath)
		} else {
			ctx.configDir = filepath.Dir(f)
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)

			// 1. 提取宿主 auth-dir
			if ctx.authDir == "" && (strings.HasPrefix(line, "auth-dir:") || strings.HasPrefix(line, "auth_dir:")) {
				val := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "auth-dir:"), "auth_dir:"))
				val = strings.Trim(val, `"' `)
				if val != "" {
					ctx.authDir = val
				}
			}

			// 2. 提取宿主 management key
			if ctx.managementKey == "" {
				if strings.HasPrefix(line, "management_key:") || strings.HasPrefix(line, "management-key:") {
					val := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "management_key:"), "management-key:"))
					val = strings.Trim(val, `"' `)
					if val != "" && !strings.HasPrefix(val, "$2a$") {
						ctx.managementKey = val
					}
				}
				if strings.HasPrefix(line, "secret-key:") || strings.HasPrefix(line, "secret_key:") {
					val := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "secret-key:"), "secret_key:"))
					val = strings.Trim(val, `"' `)
					if val != "" && !strings.HasPrefix(val, "$2a$") {
						ctx.managementKey = val
					}
				}
			}

			// 3. 提取宿主 proxy-url
			if ctx.proxyURL == "" {
				if strings.HasPrefix(line, "proxy-url:") || strings.HasPrefix(line, "proxy_url:") {
					val := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "proxy-url:"), "proxy_url:"))
					val = strings.Trim(val, `"' `)
					if val != "" {
						ctx.proxyURL = val
					}
				}
			}
		}

		if ctx.configDir != "" {
			break
		}
	}

	// 环境变量兜底
	if envAuth := os.Getenv("CPA_AUTH_DIR"); envAuth != "" {
		ctx.authDir = envAuth
	}
	if ctx.managementKey == "" {
		for _, env := range []string{"WB_MANAGEMENT_KEY", "CPA_MANAGEMENT_KEY", "MANAGEMENT_KEY"} {
			if v := strings.TrimSpace(os.Getenv(env)); v != "" {
				ctx.managementKey = v
				break
			}
		}
	}
	if ctx.proxyURL == "" {
		for _, env := range []string{"WB_PROXY_URL", "ALL_PROXY", "all_proxy", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
			if v := strings.TrimSpace(os.Getenv(env)); v != "" {
				ctx.proxyURL = v
				break
			}
		}
	}

	// 展开 ~ 符号
	if strings.HasPrefix(ctx.authDir, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			ctx.authDir = filepath.Join(home, strings.TrimPrefix(ctx.authDir, "~"))
		}
	}
	if ctx.authDir != "" && !filepath.IsAbs(ctx.authDir) && ctx.configDir != "" {
		ctx.authDir = filepath.Join(ctx.configDir, ctx.authDir)
	}

	return ctx
}

// resolvePaths 根据宿主上下文解析出绝对持久化路径，确保位于持久卷中，严禁依赖不可靠的相对当前工作目录
func resolvePaths(cfg *PluginConfig, hostCtx cpaHostContext) {
	// -------------------------------------------------------------------------
	// 1. 解析 AuthDir
	// -------------------------------------------------------------------------
	rawAuthDir := strings.TrimSpace(cfg.AuthDir)
	if rawAuthDir != "" {
		// 用户明确配置了 auth_dir
		if strings.HasPrefix(rawAuthDir, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				rawAuthDir = filepath.Join(home, strings.TrimPrefix(rawAuthDir, "~"))
			}
		}
		if !filepath.IsAbs(rawAuthDir) {
			if hostCtx.authDir != "" {
				// 相对路径优先解析到 CPA 挂载的 auth-dir 持久卷下
				rawAuthDir = filepath.Join(hostCtx.authDir, rawAuthDir)
			} else if hostCtx.configDir != "" {
				rawAuthDir = filepath.Join(hostCtx.configDir, rawAuthDir)
			}
		}
	} else if hostCtx.authDir != "" {
		// 未显式配置：自动使用 CPA 的 auth-dir 下的 workbuddy 子目录 (处于挂载持久卷中)
		rawAuthDir = filepath.Join(hostCtx.authDir, "workbuddy")
	} else if hostCtx.configDir != "" {
		rawAuthDir = filepath.Join(hostCtx.configDir, "auths")
	} else {
		rawAuthDir = "auths"
	}

	absAuth, err := filepath.Abs(rawAuthDir)
	if err == nil {
		cfg.ResolvedAuthDir = absAuth
	} else {
		cfg.ResolvedAuthDir = rawAuthDir
	}

	// -------------------------------------------------------------------------
	// 2. 解析 DataDir
	// -------------------------------------------------------------------------
	rawDataDir := strings.TrimSpace(cfg.DataDir)
	if rawDataDir != "" {
		if strings.HasPrefix(rawDataDir, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				rawDataDir = filepath.Join(home, strings.TrimPrefix(rawDataDir, "~"))
			}
		}
		if !filepath.IsAbs(rawDataDir) {
			if hostCtx.authDir != "" {
				rawDataDir = filepath.Join(hostCtx.authDir, rawDataDir)
			} else if hostCtx.configDir != "" {
				rawDataDir = filepath.Join(hostCtx.configDir, rawDataDir)
			}
		}
	} else if hostCtx.authDir != "" {
		// 默认存入持久卷下的 .codebuddy_data 目录
		rawDataDir = filepath.Join(hostCtx.authDir, ".codebuddy_data")
	} else if hostCtx.configDir != "" {
		rawDataDir = filepath.Join(hostCtx.configDir, ".codebuddy_data")
	} else {
		rawDataDir = ".codebuddy_data"
	}

	absData, err := filepath.Abs(rawDataDir)
	if err == nil {
		cfg.ResolvedDataDir = absData
	} else {
		cfg.ResolvedDataDir = rawDataDir
	}
}

// Get 返回当前插件生效的配置快照
func Get() *PluginConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	cfgCopy := *globalConfig
	return &cfgCopy
}

// GetAuthDir 获取解析后的绝对持久化账号凭证目录 (位于已挂载 Volume 下)
func GetAuthDir() string {
	cfg := Get()
	if cfg.ResolvedAuthDir != "" {
		return cfg.ResolvedAuthDir
	}
	return "auths"
}

// GetDataDir 获取解析后的绝对持久化数据目录 (位于已挂载 Volume 下)
func GetDataDir() string {
	cfg := Get()
	if cfg.ResolvedDataDir != "" {
		return cfg.ResolvedDataDir
	}
	return ".codebuddy_data"
}

// GetManagementKey 获取当前探测或配置的管理密钥
func GetManagementKey() string {
	return Get().ManagementKey
}

// GetProxyURL 获取当前生效的代理地址
func GetProxyURL() string {
	return Get().ProxyURL
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

// LoadFromMap 从 CPA 宿主传入的通用 map 解析配置并重新计算持久化路径
func LoadFromMap(raw map[string]any) error {
	rawBytes, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(rawBytes, cfg); err != nil {
		return err
	}

	// 兼容多种 key 变体
	keyAliases := []string{"management_key", "management-key", "managementKey", "secret_key", "secret-key", "secretKey", "key"}
	for _, alias := range keyAliases {
		if v, ok := raw[alias]; ok {
			if strVal, isStr := v.(string); isStr && strings.TrimSpace(strVal) != "" {
				cfg.ManagementKey = strings.TrimSpace(strVal)
				break
			}
		}
	}

	// 兼容 proxy 变体
	proxyAliases := []string{"proxy_url", "proxy-url", "proxyUrl", "proxy"}
	for _, alias := range proxyAliases {
		if v, ok := raw[alias]; ok {
			if strVal, isStr := v.(string); isStr && strings.TrimSpace(strVal) != "" {
				cfg.ProxyURL = strings.TrimSpace(strVal)
				break
			}
		}
	}

	// 兼容 auth_dir 变体
	authDirAliases := []string{"auth_dir", "auth-dir", "authDir"}
	for _, alias := range authDirAliases {
		if v, ok := raw[alias]; ok {
			if strVal, isStr := v.(string); isStr && strings.TrimSpace(strVal) != "" {
				cfg.AuthDir = strings.TrimSpace(strVal)
				break
			}
		}
	}

	// 兼容 data_dir 变体
	dataDirAliases := []string{"data_dir", "data-dir", "dataDir"}
	for _, alias := range dataDirAliases {
		if v, ok := raw[alias]; ok {
			if strVal, isStr := v.(string); isStr && strings.TrimSpace(strVal) != "" {
				cfg.DataDir = strings.TrimSpace(strVal)
				break
			}
		}
	}

	hostCtx := cpaHostContext{
		configDir:     cfg.CPAConfigDir,
		authDir:       cfg.CPAAuthDir,
		managementKey: cfg.ManagementKey,
		proxyURL:      cfg.ProxyURL,
	}
	resolvePaths(cfg, hostCtx)

	Update(cfg)
	return nil
}

// EnsureDataDir 保证运行时数据目录就绪
func (c *PluginConfig) EnsureDataDir() error {
	dir := c.ResolvedDataDir
	if dir == "" {
		dir = GetDataDir()
	}
	return os.MkdirAll(dir, 0755)
}

// EnsureAuthDir 保证账号持久化存储目录就绪
func (c *PluginConfig) EnsureAuthDir() error {
	dir := c.ResolvedAuthDir
	if dir == "" {
		dir = GetAuthDir()
	}
	return os.MkdirAll(dir, 0755)
}

// ResolvePath 解析相对于持久化数据目录的文件路径
func (c *PluginConfig) ResolvePath(elem ...string) string {
	dir := c.ResolvedDataDir
	if dir == "" {
		dir = GetDataDir()
	}
	parts := append([]string{dir}, elem...)
	return filepath.Join(parts...)
}
