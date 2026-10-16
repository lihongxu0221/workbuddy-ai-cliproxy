package executor

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
)

// UpstreamClient 具备防爬指纹伪装与代理支持的高性能 HTTP 客户端封装
type UpstreamClient struct {
	baseClient *http.Client
}

// NewUpstreamClient 创建上游客户端
func NewUpstreamClient() *UpstreamClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	return &UpstreamClient{
		baseClient: &http.Client{
			Transport: transport,
			Timeout:   120 * time.Second,
		},
	}
}

// ClientForAuth 根据账号独立代理配置获取专属 http.Client
func (c *UpstreamClient) ClientForAuth(a *auth.WorkBuddyAuth) *http.Client {
	if a == nil || a.ProxyURL == "" {
		return c.baseClient
	}

	proxyParsed, err := url.Parse(a.ProxyURL)
	if err != nil {
		return c.baseClient
	}

	customTransport := &http.Transport{
		Proxy: http.ProxyURL(proxyParsed),
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
	}

	return &http.Client{
		Transport: customTransport,
		Timeout:   120 * time.Second,
	}
}

// ApplyStandardHeaders 向发送给 WorkBuddy 上游的请求注入指纹标头
func ApplyStandardHeaders(req *http.Request, a *auth.WorkBuddyAuth) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json, text/plain, */*")

	if a == nil {
		req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
		return
	}

	if a.Realm == auth.RealmCN {
		req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
		req.Header.Set("Origin", "https://www.codebuddy.cn")
		req.Header.Set("Referer", "https://www.codebuddy.cn/")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("X-Product", "SaaS")
		req.Header.Set("X-Domain", "copilot.tencent.com")

		if a.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+a.AccessToken)
		} else {
			req.Header.Set("X-No-Authorization", "1")
		}
		if a.UserID != "" {
			req.Header.Set("X-User-Id", a.UserID)
		} else {
			req.Header.Set("X-No-User-Id", "1")
		}
		if a.RefreshToken != "" {
			req.Header.Set("X-Refresh-Token", a.RefreshToken)
		}
	} else {
		// Global / en
		req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
		req.Header.Set("Origin", "https://www.codebuddy.ai")
		req.Header.Set("Referer", "https://www.codebuddy.ai/")
		req.Header.Set("X-Domain", a.Domain())

		if a.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+a.AccessToken)
		}
		if a.UserID != "" {
			req.Header.Set("X-User-Id", a.UserID)
		}
		if a.RefreshToken != "" {
			req.Header.Set("X-Refresh-Token", a.RefreshToken)
		}
	}
}
