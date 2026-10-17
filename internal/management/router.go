package management

import (
	"net/http"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/web"

	"github.com/gin-gonic/gin"
)

// MountRoutes 在 CPA 宿主的 Gin 引擎上挂载 WorkBuddy 管理控制台与 REST API
func MountRoutes(group *gin.RouterGroup) {
	// 1. 静态资产与前端 UI 直接返回自包含 HTML，彻底杜绝 301 重定向与资源丢失
	renderUI := func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Data(http.StatusOK, "text/html; charset=utf-8", web.RenderFullHTML(config.GetManagementKey()))
	}

	if group.BasePath() == "" || group.BasePath() == "/" {
		group.GET("/", renderUI)
	} else {
		group.GET("", renderUI)
		group.GET("/", renderUI)
	}
	group.GET("/ui", renderUI)
	group.GET("/ui/", renderUI)
	group.StaticFS("/ui/static", http.FS(web.StaticFS))

	// 2. 账号体系 API (同时支持 GET 与 POST/DELETE，免受网关鉴权阻碍)
	group.GET("/accounts", HandleListAccounts)
	group.GET("/api/accounts", HandleListAccounts)
	group.POST("/accounts/import", HandleImportAccount)
	group.GET("/accounts/import", HandleImportAccount)
	group.GET("/api/accounts/import", HandleImportAccount)
	group.POST("/accounts/refresh", HandleRefreshAccount)
	group.GET("/accounts/refresh", HandleRefreshAccount)
	group.GET("/api/accounts/refresh", HandleRefreshAccount)
	group.DELETE("/accounts/:id", HandleDeleteAccount)
	group.GET("/accounts/delete", HandleDeleteAccount)
	group.GET("/api/accounts/delete", HandleDeleteAccount)

	// 3. 动作与打卡执行 API
	group.POST("/actions/dailybonus", HandleDailyBonusManual)
	group.GET("/actions/dailybonus", HandleDailyBonusManual)
	group.GET("/api/actions/dailybonus", HandleDailyBonusManual)
	group.POST("/actions/checkin", HandleCheckinManual)
	group.GET("/actions/checkin", HandleCheckinManual)
	group.GET("/api/actions/checkin", HandleCheckinManual)
	group.POST("/actions/trial", HandleActivateTrial)
	group.GET("/actions/trial", HandleActivateTrial)
	group.GET("/api/actions/trial", HandleActivateTrial)
	group.GET("/dailybonus/status", HandleDailyBonusStatus)
	group.GET("/api/dailybonus/status", HandleDailyBonusStatus)
	group.POST("/actions/login-start", HandleLoginStart)
	group.GET("/actions/login-start", HandleLoginStart)
	group.GET("/api/actions/login-start", HandleLoginStart)
	group.POST("/actions/login-poll", HandleLoginPoll)
	group.GET("/actions/login-poll", HandleLoginPoll)
	group.GET("/api/actions/login-poll", HandleLoginPoll)

	// 4. 模型管理 API
	group.GET("/models", HandleListModels)
	group.GET("/api/models", HandleListModels)
	group.POST("/models/toggle", HandleToggleModel)
	group.GET("/models/toggle", HandleToggleModel)
	group.POST("/models/test", HandleTestModel)
	group.GET("/models/test", HandleTestModel)
	group.GET("/api/models/test", HandleTestModel)

	// 5. 统计与配置 API
	group.GET("/stats", HandleGetStats)
	group.GET("/api/stats", HandleGetStats)
	group.GET("/config", HandleGetConfig)
	group.POST("/config", HandleUpdateConfig)
}
