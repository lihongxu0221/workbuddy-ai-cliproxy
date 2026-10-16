package auth

import (
	"sync"
	"time"
)

// Realm 定义 WorkBuddy 区域分类
type Realm string

const (
	// RealmCN 国内区 (copilot.tencent.com / workbuddy.cn)
	RealmCN Realm = "cn"
	// RealmGlobal 国际区 (workbuddy.ai / codebuddy.ai)
	RealmGlobal Realm = "global"
)

// CreditPackage 积分包明细
type CreditPackage struct {
	PackageCode      string `json:"packageCode"`
	PackageName      string `json:"packageName"`
	RemainingCredits float64 `json:"remainingCredits"`
	InitialCredits   float64 `json:"initialCredits"`
	EffectiveTime    int64   `json:"effectiveTime"` // Unix 毫秒
	ExpireTime       int64   `json:"expireTime"`    // Unix 毫秒
	Status           int     `json:"status"`        // 1=生效中, 2=已过期/耗尽
}

// WorkBuddyAuth 单个 WorkBuddy 账号凭据数据结构
type WorkBuddyAuth struct {
	// Type 认证类型，固定为 "workbuddy"
	Type string `json:"type"`
	// ID 凭据唯一标识，如 "workbuddy-user123-intl"
	ID string `json:"id"`
	// Realm 区域: "cn" 或 "global"
	Realm Realm `json:"realm"`
	// AccessToken 访问令牌 (JWT)
	AccessToken string `json:"access_token"`
	// RefreshToken 刷新令牌
	RefreshToken string `json:"refresh_token,omitempty"`
	// ExpiresAt 访问令牌过期时间戳 (Unix 秒)
	ExpiresAt int64 `json:"expires_at"`
	// UserID 上游分配的用户唯一 ID
	UserID string `json:"user_id"`
	// EnterpriseID 企业 ID (国内区多租户)
	EnterpriseID string `json:"enterprise_id,omitempty"`
	// Email 用户邮箱/账号标识
	Email string `json:"email,omitempty"`
	// Nickname 用户显示昵称
	Nickname string `json:"nickname,omitempty"`
	// Credits 当前可用总积分余额
	Credits float64 `json:"credits"`
	// CreditPackages 详细积分包列表
	CreditPackages []CreditPackage `json:"credit_packages,omitempty"`
	// Disabled 手动禁用标记
	Disabled bool `json:"disabled"`
	// ProxyURL 绑定的专用代理地址 (如 "http://127.0.0.1:7890")
	ProxyURL string `json:"proxy_url,omitempty"`
	// FilePath 对应磁盘文件路径
	FilePath string `json:"-"`

	mu sync.RWMutex
}

// IsExpired 判断访问令牌是否已临近过期 (预留 5 分钟缓冲)
func (a *WorkBuddyAuth) IsExpired() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > (a.ExpiresAt - 300)
}

// IsGlobal 是否为国际区账号
func (a *WorkBuddyAuth) IsGlobal() bool {
	return a.Realm == RealmGlobal
}

// Domain 获取该区域对应的主通信域名
func (a *WorkBuddyAuth) Domain() string {
	if a.IsGlobal() {
		return "codebuddy.ai"
	}
	return "copilot.tencent.com"
}

// BaseURL 获取该区域对应的 API 网关基础地址
func (a *WorkBuddyAuth) BaseURL() string {
	if a.IsGlobal() {
		return "https://www.codebuddy.ai"
	}
	return "https://copilot.tencent.com"
}

// Copy 浅拷贝凭据对象
func (a *WorkBuddyAuth) Copy() *WorkBuddyAuth {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return &WorkBuddyAuth{
		Type:           a.Type,
		ID:             a.ID,
		Realm:          a.Realm,
		AccessToken:    a.AccessToken,
		RefreshToken:   a.RefreshToken,
		ExpiresAt:      a.ExpiresAt,
		UserID:         a.UserID,
		EnterpriseID:   a.EnterpriseID,
		Email:          a.Email,
		Nickname:       a.Nickname,
		Credits:        a.Credits,
		CreditPackages: a.CreditPackages,
		Disabled:       a.Disabled,
		ProxyURL:       a.ProxyURL,
		FilePath:       a.FilePath,
	}
}
