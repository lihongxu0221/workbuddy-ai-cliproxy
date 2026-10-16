package usage

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"
	"workbuddy-ai-cliproxy/internal/config"
)

// SummaryStats 聚合指标视图
type SummaryStats struct {
	TotalRequests    int64   `json:"total_requests"`
	TodayRequests    int64   `json:"today_requests"`
	TotalTokens      int64   `json:"total_tokens"`
	CachedTokens     int64   `json:"cached_tokens"`
	CacheHitRate     float64 `json:"cache_hit_rate"` // 百分比
	TotalCreditSpent float64 `json:"total_credit_spent"`
}

// StatsCollector 统计汇总收集器
type StatsCollector struct {
	mu sync.RWMutex
}

var defaultCollector = &StatsCollector{}

// GetCollector 获取统计收集器
func GetCollector() *StatsCollector {
	return defaultCollector
}

// ComputeSummary 从当前 JSONL 快速聚合并返回统计
func (c *StatsCollector) ComputeSummary() SummaryStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfg := config.Get()
	filePath := cfg.ResolvePath("request-usage.jsonl")

	var stats SummaryStats
	f, err := os.Open(filePath)
	if err != nil {
		return stats
	}
	defer f.Close()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}

		stats.TotalRequests++
		if r.Timestamp >= todayStart {
			stats.TodayRequests++
		}
		stats.TotalTokens += int64(r.PromptTokens + r.CompletionTokens)
		stats.CachedTokens += int64(r.CachedTokens)
		stats.TotalCreditSpent += r.Credit
	}

	if stats.TotalTokens > 0 {
		stats.CacheHitRate = float64(stats.CachedTokens) / float64(stats.TotalTokens) * 100.0
	}

	return stats
}
