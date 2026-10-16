package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/internal/executor"
)

// Record 单次请求审计流水记录 (去隐私元数据)
type Record struct {
	Timestamp        int64   `json:"timestamp"` // Unix 秒
	AccountID        string  `json:"account_id"`
	Realm            string  `json:"realm"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	Credit           float64 `json:"credit"`
	DurationMs       int64   `json:"duration_ms"`
	Success          bool    `json:"success"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

// Logger 负责高效轮转写入 JSONL 审计流水
type Logger struct {
	filePath string
	mu       sync.Mutex
}

var (
	defaultLogger *Logger
	onceLogger    sync.Once
)

// GetLogger 获取审计日志记录器单例
func GetLogger() *Logger {
	onceLogger.Do(func() {
		cfg := config.Get()
		_ = cfg.EnsureDataDir()
		defaultLogger = &Logger{
			filePath: cfg.ResolvePath("request-usage.jsonl"),
		}
	})
	return defaultLogger
}

// LogRequest 记录一次请求元数据
func (l *Logger) LogRequest(accID, realm, model string, usage *executor.UsageInfo, durationMs int64, success bool, errMsg string) {
	rec := Record{
		Timestamp:    time.Now().Unix(),
		AccountID:    accID,
		Realm:        realm,
		Model:        model,
		DurationMs:   durationMs,
		Success:      success,
		ErrorMessage: errMsg,
	}

	if usage != nil {
		rec.PromptTokens = usage.PromptTokens
		rec.CompletionTokens = usage.CompletionTokens
		rec.CachedTokens = usage.CachedTokens
		rec.Credit = usage.Credit
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.checkRotateLocked()

	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(append(data, '\n'))
}

func (l *Logger) checkRotateLocked() {
	fi, err := os.Stat(l.filePath)
	if err != nil {
		return
	}
	// 当文件超过 50MB 时自动重命名归档
	if fi.Size() > 50*1024*1024 {
		archiveName := fmt.Sprintf("%s.%s", l.filePath, time.Now().Format("20060102-150405"))
		_ = os.Rename(l.filePath, archiveName)
	}
}
