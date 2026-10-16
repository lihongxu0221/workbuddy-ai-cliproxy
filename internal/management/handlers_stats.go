package management

import (
	"net/http"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/internal/usage"

	"github.com/gin-gonic/gin"
)

// HandleGetStats 获取用量审计聚合指标
func HandleGetStats(c *gin.Context) {
	collector := usage.GetCollector()
	summary := collector.ComputeSummary()

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": summary,
	})
}

// HandleGetConfig 获取当前运行时配置
func HandleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": config.Get(),
	})
}

// HandleUpdateConfig 动态热更新插件配置
func HandleUpdateConfig(c *gin.Context) {
	var body config.PluginConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	config.Update(&body)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "config updated successfully",
		"data":    config.Get(),
	})
}
