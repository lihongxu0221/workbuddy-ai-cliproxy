package management

import (
	"net/http"
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

// HandleImportAccount 导入新凭据
func HandleImportAccount(c *gin.Context) {
	var body struct {
		Token        string     `json:"token" binding:"required"`
		RefreshToken string     `json:"refresh_token"`
		UserID       string     `json:"user_id"`
		Nickname     string     `json:"nickname"`
		Realm        auth.Realm `json:"realm"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	helper := auth.NewLoginHelper()
	acc, err := helper.ImportToken(c.Request.Context(), body.Token, body.RefreshToken, body.UserID, body.Nickname, body.Realm)
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

// HandleRefreshAccount 触发指定账号余额与 Token 刷新
func HandleRefreshAccount(c *gin.Context) {
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

// HandleDeleteAccount 删除账号
func HandleDeleteAccount(c *gin.Context) {
	id := c.Param("id")
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
