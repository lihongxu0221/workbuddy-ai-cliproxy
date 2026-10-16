package management

import (
	"net/http"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/web"

	"github.com/gin-gonic/gin"
)

// MountRoutes 在 CPA 宿主的 Gin 引擎上挂载 WorkBuddy 管理控制台与 REST API
func MountRoutes(group *gin.RouterGroup) {
	// 鉴权中间件 (可选检查 ManagementKey)
	group.Use(func(c *gin.Context) {
		cfgKey := config.Get().ManagementKey
		if cfgKey != "" {
			reqKey := c.GetHeader("Authorization")
			if reqKey != "Bearer "+cfgKey && reqKey != cfgKey {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid management key"})
				return
			}
		}
		c.Next()
	})

	// 1. 静态资产与前端 UI 挂载
	group.StaticFS("/ui", http.FS(web.StaticFS))
	group.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, c.Request.URL.Path+"/ui/")
	})

	// 2. 账号体系 API
	group.GET("/accounts", HandleListAccounts)
	group.POST("/accounts/import", HandleImportAccount)
	group.POST("/accounts/refresh", HandleRefreshAccount)
	group.DELETE("/accounts/:id", HandleDeleteAccount)

	// 3. 动作与打卡执行 API
	group.POST("/actions/dailybonus", HandleDailyBonusManual)
	group.POST("/actions/checkin", HandleCheckinManual)
	group.POST("/actions/trial", HandleActivateTrial)
	group.GET("/dailybonus/status", HandleDailyBonusStatus)

	// 4. 模型管理 API
	group.GET("/models", HandleListModels)
	group.POST("/models/toggle", HandleToggleModel)
	group.POST("/models/test", HandleTestModel)

	// 5. 统计与配置 API
	group.GET("/stats", HandleGetStats)
	group.GET("/config", HandleGetConfig)
	group.POST("/config", HandleUpdateConfig)
}
