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
	Plan           string               `json:"plan"`
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
		planName := a.Plan
		if planName == "" {
			planName = "免费版"
		}
		dtoList = append(dtoList, AccountDTO{
			ID:             a.ID,
			Realm:          a.Realm,
			UserID:         a.UserID,
			Email:          a.Email,
			Nickname:       a.Nickname,
			Plan:           planName,
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

// HandleImportAccount 导入新凭据 (严格要求 POST JSON 载荷)
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

	token := strings.TrimSpace(body.Token)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "token is required"})
		return
	}

	realm := body.Realm
	if realm == "" {
		realm = auth.RealmGlobal
	}

	helper := auth.NewLoginHelper()
	acc, err := helper.ImportToken(c.Request.Context(), token, strings.TrimSpace(body.RefreshToken), strings.TrimSpace(body.UserID), strings.TrimSpace(body.Nickname), realm)
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

// HandleRefreshAccount 触发指定账号余额与 Token 刷新 (严格要求 POST JSON 载荷)
func HandleRefreshAccount(c *gin.Context) {
	var body struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	id := strings.TrimSpace(body.ID)
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "id is required"})
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

	planName := acc.Plan
	if planName == "" {
		planName = "免费版"
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":       acc.ID,
			"plan":     planName,
			"credits":  credits,
			"packages": pkgs,
		},
	})
}

// HandleDeleteAccount 删除账号
// 主路径：POST /accounts/delete (id 放 JSON 请求体) —— CPA v8 插件路由只做精确匹配，不支持 :id 参数，
// 因此与 import / refresh 保持一致的固定路径写法；独立 CLI 模式下仍兼容 DELETE /accounts/:id。
func HandleDeleteAccount(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			id = strings.TrimSpace(body.ID)
		}
	}
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "missing account id (JSON body field 'id')"})
		return
	}

	if err := auth.GetStore().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted", "data": gin.H{"id": id}})
}
