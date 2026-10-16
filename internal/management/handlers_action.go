package management

import (
	"net/http"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/growth"

	"github.com/gin-gonic/gin"
)

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
