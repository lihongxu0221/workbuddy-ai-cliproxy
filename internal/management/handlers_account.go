package management

import (
	"net/http"
	"strings"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/failover"

	"github.com/gin-gonic/gin"
)

// AccountDTO 返回给前端控制台的账号脱敏结构体
type AccountDTO struct {
	ID             string               `json:"id"`
	Realm          auth.Realm           `json:"realm"`
	UserID         string               `json:"user_id"`
	Email          string               `json:"email"`
	Nickname       string               `json:"nickname"`
	Credits        float64              `json:"credits"`
	CreditPackages []auth.CreditPackage `json:"credit_packages"`
	Disabled       bool                 `json:"disabled"`
	InFlightSlots  int                  `json:"in_flight_slots"`
	IsQuarantined  bool                 `json:"is_quarantined"`
	QuarantineInfo string               `json:"quarantine_info,omitempty"`
}

// HandleListAccounts 获取所有账号列表与实时状态
func HandleListAccounts(c *gin.Context) {
	store := auth.GetStore()
	gate := failover.GetGate()
	quarantine := failover.GetQuarantine()

	rawList := store.List()
	dtoList := make([]AccountDTO, 0, len(rawList))

	for _, a := range rawList {
		isQ, qReason := quarantine.IsQuarantined(a.ID, "")
		dtoList = append(dtoList, AccountDTO{
			ID:             a.ID,
			Realm:          a.Realm,
			UserID:         a.UserID,
			Email:          a.Email,
			Nickname:       a.Nickname,
			Credits:        a.Credits,
			CreditPackages: a.CreditPackages,
			Disabled:       a.Disabled,
			InFlightSlots:  gate.InFlightCount(a.ID),
			IsQuarantined:  isQ,
			QuarantineInfo: qReason,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": dtoList,
	})
}

// HandleImportAccount 导入新凭据 (同时兼容 JSON Body 与 Query 参数)
func HandleImportAccount(c *gin.Context) {
	var token, refreshToken, userID, nickname string
	realm := auth.RealmGlobal

	var body struct {
		Token        string     `json:"token"`
		RefreshToken string     `json:"refresh_token"`
		UserID       string     `json:"user_id"`
		Nickname     string     `json:"nickname"`
		Realm        auth.Realm `json:"realm"`
	}

	if err := c.ShouldBindJSON(&body); err == nil && body.Token != "" {
		token = strings.TrimSpace(body.Token)
		refreshToken = strings.TrimSpace(body.RefreshToken)
		userID = strings.TrimSpace(body.UserID)
		nickname = strings.TrimSpace(body.Nickname)
		if body.Realm != "" {
			realm = body.Realm
		}
	} else {
		token = strings.TrimSpace(c.Query("token"))
		refreshToken = strings.TrimSpace(c.Query("refresh_token"))
		userID = strings.TrimSpace(c.Query("user_id"))
		nickname = strings.TrimSpace(c.Query("nickname"))
		if r := c.Query("realm"); r != "" {
			realm = auth.Realm(r)
		}
	}

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "missing token"})
		return
	}

	helper := auth.NewLoginHelper()
	acc, err := helper.ImportToken(c.Request.Context(), token, refreshToken, userID, nickname, realm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "account imported successfully",
		"data":    acc,
	})
}

// HandleRefreshAccount 触发指定账号余额与 Token 刷新 (同时兼容 JSON Body 与 Query 参数)
func HandleRefreshAccount(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = c.ShouldBindJSON(&body)
		id = strings.TrimSpace(body.ID)
	}

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

	checker := auth.NewBalanceChecker()
	credits, pkgs, err := checker.FetchBalance(c.Request.Context(), acc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":       acc.ID,
			"credits":  credits,
			"packages": pkgs,
		},
	})
}

// HandleDeleteAccount 删除账号 (同时兼容 Path Param 与 Query 参数)
func HandleDeleteAccount(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		id = strings.TrimSpace(c.Query("id"))
	}
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = c.ShouldBindJSON(&body)
		id = strings.TrimSpace(body.ID)
	}

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "missing id"})
		return
	}

	if err := auth.GetStore().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}
