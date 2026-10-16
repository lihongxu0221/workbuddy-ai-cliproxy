package management

import (
	"encoding/json"
	"net/http"
	"workbuddy-ai-cliproxy/internal/executor"
	"workbuddy-ai-cliproxy/internal/failover"
	"workbuddy-ai-cliproxy/internal/models"

	"github.com/gin-gonic/gin"
)

// HandleListModels 返回所有模型信息
func HandleListModels(c *gin.Context) {
	reg := models.GetRegistry()
	list := reg.ListAll()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": list,
	})
}

// HandleToggleModel 启用或下架模型
func HandleToggleModel(c *gin.Context) {
	var body struct {
		ID       string `json:"id" binding:"required"`
		Disabled bool   `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	reg := models.GetRegistry()
	reg.SetDisabled(body.ID, body.Disabled)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "updated"})
}

// HandleTestModel 发送一条轻量真实对话以测试连通性并探查真实 Credit 扣费
func HandleTestModel(c *gin.Context) {
	var body struct {
		Model string `json:"model" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	parsed := models.ParsePrefix(body.Model)
	pipeline := failover.NewPipeline(executor.NewWorkBuddyExecutor())

	testReq := map[string]any{
		"model": parsed.CleanModel,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"stream":     false,
		"max_tokens": 16,
	}
	payloadBytes, _ := json.Marshal(testReq)

	resp, err := pipeline.ExecuteWithFailover(c.Request.Context(), "", parsed.TargetRealm, parsed.CleanModel, "test-ping", executor.Request{
		Model:   parsed.CleanModel,
		Payload: payloadBytes,
	})

	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    502,
			"error":   err.Error(),
			"message": "model connectivity test failed",
		})
		return
	}

	var parsedResp map[string]any
	_ = json.Unmarshal(resp.Payload, &parsedResp)

	c.JSON(http.StatusOK, gin.H{
		"code":     0,
		"message":  "connectivity test passed",
		"model":    parsed.CleanModel,
		"realm":    parsed.TargetRealm,
		"response": parsedResp,
		"usage":    resp.Usage,
	})
}
