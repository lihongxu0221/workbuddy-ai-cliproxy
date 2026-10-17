package growth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
)

// GlobalBonusReporter 国际区每日 30 积分打卡上报器
type GlobalBonusReporter struct {
	client *http.Client
}

// NewGlobalBonusReporter 实例化打卡上报器
func NewGlobalBonusReporter() *GlobalBonusReporter {
	return &GlobalBonusReporter{
		client: config.NewHTTPClient(25*time.Second, ""),
	}
}

type heatmapResp struct {
	Code int `json:"code"`
	Data struct {
		Records []struct {
			Date  string `json:"date"`
			Score int    `json:"score"`
		} `json:"records"`
		StreakDays int `json:"streakDays"`
	} `json:"data"`
}

// CheckTodayLit 检查今日是否已经在成长中心点亮 (Score > 0)
func (r *GlobalBonusReporter) CheckTodayLit(ctx context.Context, acc *auth.WorkBuddyAuth) (bool, int, error) {
	if !acc.IsGlobal() {
		return false, 0, fmt.Errorf("account %s is not global realm", acc.ID)
	}

	apiURL := fmt.Sprintf("%s/activity/growth/heatmap", acc.BaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, 0, err
	}

	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
	req.Header.Set("X-Domain", acc.Domain())
	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return false, 0, fmt.Errorf("get heatmap failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed heatmapResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return false, 0, err
	}

	if len(parsed.Data.Records) == 0 {
		return false, 0, nil
	}

	// 最后一个记录即代表今日单元格
	lastCell := parsed.Data.Records[len(parsed.Data.Records)-1]
	return lastCell.Score > 0, lastCell.Score, nil
}

type reportEvent struct {
	EventCode            string `json:"eventCode"`
	Timestamp            int64  `json:"timestamp"`
	Mode                 string `json:"mode"`
	ConversationID       string `json:"conversationId"`
	RequestID            string `json:"requestId"`
	InputLength          int    `json:"inputLength"`
	RequestModelID       string `json:"requestModelId"`
	RequestModelName     string `json:"requestModelName"`
	PresentAt            int64  `json:"presentAt"`
	RootRequestID        string `json:"rootRequestId"`
	ParentConversationID string `json:"parentConversationId"`
	AgentName            string `json:"agentName"`
	AgentType            string `json:"agentType"`
	UserID               string `json:"userId"`
}

// ReportActivity 执行一次活跃打卡上报
func (r *GlobalBonusReporter) ReportActivity(ctx context.Context, acc *auth.WorkBuddyAuth) error {
	if !acc.IsGlobal() {
		return fmt.Errorf("account %s is not global realm", acc.ID)
	}

	// 先行检查今日是否已点亮，若已点亮则跳过，杜绝多余上报
	lit, score, err := r.CheckTodayLit(ctx, acc)
	if err == nil && lit {
		return nil
	}

	nowMs := time.Now().UnixMilli()
	events := []reportEvent{
		{
			EventCode:            "chat_request_send",
			Timestamp:            nowMs,
			Mode:                 "craft",
			ConversationID:       "wb-auto-report",
			RequestID:            "",
			InputLength:          16,
			RequestModelID:       "deepseek-v4-flash",
			RequestModelName:     "DeepSeek V4 Flash",
			PresentAt:            nowMs,
			RootRequestID:        "",
			ParentConversationID: "wb-auto-report",
			AgentName:            "default",
			AgentType:            "conversation",
			UserID:               acc.UserID,
		},
	}

	bodyBytes, err := json.Marshal(events)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/v2/report", acc.BaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	req.Header.Set("User-Agent", "WorkBuddy/1.2.8 (Windows NT 10.0; Win64; x64) Electron/32.2.1")
	req.Header.Set("X-Domain", acc.Domain())
	if acc.UserID != "" {
		req.Header.Set("X-User-Id", acc.UserID)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send report event failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("report upstream status: %d (previous score: %d)", resp.StatusCode, score)
	}

	return nil
}
