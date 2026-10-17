package management

import (
	"net/http"
	"strings"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/web"

	"github.com/gin-gonic/gin"
)

// MountRoutes 在 CPA 宿主的 Gin 引擎上挂载 WorkBuddy 管理控制台与 REST API
func MountRoutes(group *gin.RouterGroup) {
	// 1. 安全鉴权中间件：严格保护所有数据接口，拒绝未授权访问
	group.Use(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 允许直接加载 UI 页面资产
		if path == "" || path == "/" ||
			strings.HasSuffix(path, "/ui") ||
			strings.HasSuffix(path, "/ui/") ||
			strings.Contains(path, "/ui/static") {
			c.Next()
			return
		}

		// 严格校验宿主 Management Key (若配置或探测到密钥，强制鉴权)
		cfgKey := config.GetManagementKey()
		if cfgKey != "" {
			reqKey := c.GetHeader("Authorization")
			if reqKey == "" {
				reqKey = c.GetHeader("X-Management-Key")
			}

			if reqKey != "Bearer "+cfgKey && reqKey != cfgKey {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"error":   "invalid management key",
					"message": "unauthorized: management key required for this operation",
				})
				return
			}
		}
		c.Next()
	})

	// 2. 静态页面直出渲染
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

	// 3. 账号体系 API (严格遵循 HTTP Method 语义，严禁通过 GET 执行删改)
	group.GET("/accounts", HandleListAccounts)
	group.POST("/accounts/import", HandleImportAccount)
	group.POST("/accounts/refresh", HandleRefreshAccount)
	group.DELETE("/accounts/:id", HandleDeleteAccount)

	// 4. 动作与打卡执行 API (严格限制通过 POST 执行改动)
	group.POST("/actions/dailybonus", HandleDailyBonusManual)
	group.POST("/actions/checkin", HandleCheckinManual)
	group.POST("/actions/trial", HandleActivateTrial)
	group.GET("/dailybonus/status", HandleDailyBonusStatus)
	group.POST("/actions/login-start", HandleLoginStart)
	group.GET("/actions/login-poll", HandleLoginPoll)

	// 5. 模型管理 API
	group.GET("/models", HandleListModels)
	group.POST("/models/toggle", HandleToggleModel)
	group.POST("/models/test", HandleTestModel)

	// 6. 统计与配置 API
	group.GET("/stats", HandleGetStats)
	group.GET("/config", HandleGetConfig)
	group.POST("/config", HandleUpdateConfig)
}
