package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// PluginConfig 插件全局可配置项
type PluginConfig struct {
	// DailyBonus 是否启用国际区每日 30 积分成长打卡 (默认 true)
	DailyBonus bool `json:"daily_bonus" yaml:"daily_bonus"`
	// CheckinAuto 是否启用国内区每日 100 积分自动签到 (默认 true)
	CheckinAuto bool `json:"checkin_auto" yaml:"checkin_auto"`
	// MaxConcurrency 单账号最大活跃并发请求槽位数 (默认 2)
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
	// ManagementKey 管理 API 访问鉴权密钥
	ManagementKey string `json:"management_key" yaml:"management_key"`
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
	return &PluginConfig{
		DailyBonus:      true,
		CheckinAuto:     true,
		MaxConcurrency:  2,
		CNPrefix:        "cn/",
		GlobalPrefix:    "cb/",
		SessionAffinity: true,
		CacheTrace:      os.Getenv("WB_CACHE_TRACE") == "1",
		DataDir:         dataDir,
		ManagementKey:   os.Getenv("WB_MANAGEMENT_KEY"),
		SelfURL:         os.Getenv("WB_SELF_URL"),
	}
}

// Get 返回当前插件生效的配置快照
func Get() *PluginConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	cfgCopy := *globalConfig
	return &cfgCopy
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
	bytes, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(bytes, cfg); err != nil {
		return err
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
