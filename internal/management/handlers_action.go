package management

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/growth"

	"github.com/gin-gonic/gin"
)

var (
	actionLoginStates sync.Map // state string -> *actionLoginCtx
)

type actionLoginCtx struct {
	realm     auth.Realm
	expiresAt time.Time
}

// HandleDailyBonusManual 手动触发国际区成长中心打卡
func HandleDailyBonusManual(c *gin.Context) {
	var body struct {
		ID string `json:"id"` // 若为空则打卡全部 Global 账号
	}
	_ = c.ShouldBindJSON(&body)

	reporter := growth.NewGlobalBonusReporter()
	store := auth.GetStore()

	if body.ID != "" {
		acc := store.GetByID(body.ID)
		if acc == nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "account not found"})
			return
		}
		if err := reporter.ReportActivity(c.Request.Context(), acc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "reported successfully"})
		return
	}

	// 批量异步触发
	go growth.NewGlobalBonusReporter().ReportActivity(c.Request.Context(), nil)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "batch daily report triggered"})
}

// HandleCheckinManual 手动触发国内区每日签到
func HandleCheckinManual(c *gin.Context) {
	var body struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	store := auth.GetStore()
	acc := store.GetByID(body.ID)
	if acc == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "account not found"})
		return
	}

	executor := growth.NewCNCheckinExecutor()
	reward, err := executor.Checkin(c.Request.Context(), acc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":           0,
		"message":        "checkin succeeded",
		"reward_credits": reward,
	})
}

// HandleActivateTrial 手动为国际区新账号激活 250 积分 Expert 权益
func HandleActivateTrial(c *gin.Context) {
	var body struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	store := auth.GetStore()
	acc := store.GetByID(body.ID)
	if acc == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "account not found"})
		return
	}

	activator := growth.NewExpertTrialActivator()
	reward, err := activator.ActivateExpertTrial(c.Request.Context(), acc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":           0,
		"message":        "expert trial activated",
		"credit_awarded": reward,
	})
}

// HandleDailyBonusStatus 查询指定账号今日打卡与热力图状态
func HandleDailyBonusStatus(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "missing id"})
		return
	}

	store := auth.GetStore()
	acc := store.GetByID(id)
	if acc == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "account not found"})
		return
	}

	reporter := growth.NewGlobalBonusReporter()
	lit, score, err := reporter.CheckTodayLit(c.Request.Context(), acc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":    id,
			"lit":   lit,
			"score": score,
		},
	})
}

// HandleLoginStart 启动国内区或国际区的扫码/OAuth 流程
func HandleLoginStart(c *gin.Context) {
	realmStr := strings.ToLower(c.DefaultQuery("realm", "global"))
	var body struct {
		Realm string `json:"realm"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.Realm != "" {
		realmStr = strings.ToLower(body.Realm)
	}

	realm := auth.RealmGlobal
	endpoint := "https://www.codebuddy.ai/v2/plugin/auth/state?platform=CLI"
	origin := "https://www.codebuddy.ai"
	if realmStr == "cn" {
		realm = auth.RealmCN
		endpoint = "https://copilot.tencent.com/v2/plugin/auth/state?platform=CLI"
		origin = "https://www.codebuddy.cn"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			State   string `json:"state"`
			AuthURL string `json:"authUrl"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "error": err.Error()})
		return
	}

	if env.Code != 0 || env.Data.State == "" || env.Data.AuthURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": env.Code, "error": env.Msg})
		return
	}

	actionLoginStates.Store(env.Data.State, &actionLoginCtx{
		realm:     realm,
		expiresAt: time.Now().Add(5 * time.Minute),
	})

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"state":   env.Data.State,
			"authUrl": env.Data.AuthURL,
			"realm":   realm,
		},
	})
}

// HandleLoginPoll 轮询国内区或国际区的登录结果
func HandleLoginPoll(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		var body struct {
			State string `json:"state"`
		}
		_ = c.ShouldBindJSON(&body)
		state = body.State
	}

	state = strings.TrimSpace(state)
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "missing state parameter"})
		return
	}

	val, ok := actionLoginStates.Load(state)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "state expired or invalid"})
		return
	}
	ctxData := val.(*actionLoginCtx)
	if time.Now().After(ctxData.expiresAt) {
		actionLoginStates.Delete(state)
		c.JSON(http.StatusGone, gin.H{"code": 410, "error": "login session expired"})
		return
	}

	baseURL := "https://www.codebuddy.ai"
	origin := "https://www.codebuddy.ai"
	if ctxData.realm == auth.RealmCN {
		baseURL = "https://copilot.tencent.com"
		origin = "https://www.codebuddy.cn"
	}

	tokenURL := fmt.Sprintf("%s/v2/plugin/auth/token?state=%s", baseURL, state)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, tokenURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "status": "pending", "message": "waiting for network response"})
		return
	}
	defer resp.Body.Close()

	var tokenEnv struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int64  `json:"expiresIn"`
			Domain       string `json:"domain"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenEnv); err != nil || tokenEnv.Data.AccessToken == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"status":  "pending",
			"message": "waiting for scan / confirmation",
		})
		return
	}

	// 登录成功，拉取用户信息
	acctURL := fmt.Sprintf("%s/v2/plugin/login/account?state=%s", baseURL, state)
	acctReq, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, acctURL, nil)
	acctReq.Header.Set("Authorization", "Bearer "+tokenEnv.Data.AccessToken)
	acctReq.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	acctReq.Header.Set("Origin", origin)

	var uid, nickname, enterpriseID string
	if acctResp, errAcct := client.Do(acctReq); errAcct == nil {
		defer acctResp.Body.Close()
		var acctEnv struct {
			Data struct {
				UID          string `json:"uid"`
				EnterpriseID string `json:"enterpriseId"`
				Nickname     string `json:"nickname"`
			} `json:"data"`
		}
		_ = json.NewDecoder(acctResp.Body).Decode(&acctEnv)
		uid = acctEnv.Data.UID
		nickname = acctEnv.Data.Nickname
		enterpriseID = acctEnv.Data.EnterpriseID
	}

	accID := fmt.Sprintf("workbuddy-%s-%s", uid, ctxData.realm)
	if uid == "" {
		accID = fmt.Sprintf("workbuddy-%s-%d", ctxData.realm, time.Now().Unix()%100000)
	}

	acc := &auth.WorkBuddyAuth{
		Type:         "workbuddy",
		ID:           accID,
		Realm:        ctxData.realm,
		AccessToken:  tokenEnv.Data.AccessToken,
		RefreshToken: tokenEnv.Data.RefreshToken,
		ExpiresAt:    time.Now().Unix() + tokenEnv.Data.ExpiresIn,
		UserID:       uid,
		EnterpriseID: enterpriseID,
		Nickname:     nickname,
	}

	checker := auth.NewBalanceChecker()
	credits, pkgs, _ := checker.FetchBalance(context.Background(), acc)
	acc.Credits = credits
	acc.CreditPackages = pkgs

	_ = auth.GetStore().Save(acc)
	actionLoginStates.Delete(state)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"status":  "success",
		"message": "login successful and account saved",
		"data":    acc,
	})
}
