package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// LoginHelper 登录引导与凭证导入助手
type LoginHelper struct{}

// NewLoginHelper 实例化登录助手
func NewLoginHelper() *LoginHelper {
	return &LoginHelper{}
}

// ImportToken 直接通过 Token 与 UID 导入并验证生成凭据
func (h *LoginHelper) ImportToken(ctx context.Context, token, refreshToken, uid, nickname string, realm Realm) (*WorkBuddyAuth, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	if realm != RealmCN && realm != RealmGlobal {
		realm = RealmGlobal
	}

	accID := fmt.Sprintf("workbuddy-%s-%s", uid, realm)
	if uid == "" {
		accID = fmt.Sprintf("workbuddy-%d-%s", time.Now().Unix()%100000, realm)
	}

	acc := &WorkBuddyAuth{
		Type:         "workbuddy",
		ID:           accID,
		Realm:        realm,
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Unix() + 86400*30, // 默认 30 天有效期
		UserID:       uid,
		Nickname:     nickname,
	}

	// 立即拉取一次余额和积分包以验证有效性
	checker := NewBalanceChecker()
	credits, pkgs, err := checker.FetchBalance(ctx, acc)
	if err == nil {
		acc.Credits = credits
		acc.CreditPackages = pkgs
	}

	if err := GetStore().Save(acc); err != nil {
		return nil, fmt.Errorf("save imported account failed: %w", err)
	}

	return acc, nil
}
