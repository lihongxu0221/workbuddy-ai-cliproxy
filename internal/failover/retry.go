package failover

import (
	"context"
	"fmt"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/executor"
)

// Pipeline 容灾与智能调度执行管线
type Pipeline struct {
	exec *executor.WorkBuddyExecutor
}

// NewPipeline 实例化容灾管线
func NewPipeline(exec *executor.WorkBuddyExecutor) *Pipeline {
	if exec == nil {
		exec = executor.NewWorkBuddyExecutor()
	}
	return &Pipeline{exec: exec}
}

// ExecuteWithFailover 带有软会话粘性、并发准入与同 Realm 换号重试的非流式调度
func (p *Pipeline) ExecuteWithFailover(ctx context.Context, preferredID string, realm auth.Realm, model, sessionKey string, req executor.Request) (executor.Response, error) {
	gate := GetGate()
	affinity := GetAffinity()
	quarantine := GetQuarantine()
	store := auth.GetStore()

	accounts := store.ListByRealm(realm, false)
	if len(accounts) == 0 {
		// 若按 realm 未查到，尝试获取全部可用账号
		accounts = store.List()
	}
	if len(accounts) == 0 {
		return executor.Response{}, fmt.Errorf("no active accounts available in realm: %s", realm)
	}

	ordered := p.orderAccounts(accounts, preferredID, sessionKey, model, affinity, quarantine)
	var lastErr error

	for _, acc := range ordered {
		if !gate.TryAcquire(acc.ID) {
			continue // 槽位满，平滑尝试下一个
		}

		resp, err := p.exec.Execute(ctx, acc, req)
		gate.Release(acc.ID)

		if err == nil {
			// 成功执行，更新会话粘性
			if sessionKey != "" {
				affinity.BindSession(sessionKey, acc.ID)
			}
			return resp, nil
		}

		lastErr = err
		p.handleError(acc, model, err, quarantine)
	}

	if lastErr != nil {
		return executor.Response{}, fmt.Errorf("all accounts in realm %s exhausted or congested: %w", realm, lastErr)
	}
	return executor.Response{}, fmt.Errorf("no accounts in realm %s could accept request (concurrency limit reached)", realm)
}

// ExecuteStreamWithFailover 带有软会话粘性、并发准入与同 Realm 换号重试的流式调度
func (p *Pipeline) ExecuteStreamWithFailover(ctx context.Context, preferredID string, realm auth.Realm, model, sessionKey string, req executor.Request) (<-chan executor.StreamChunk, *auth.WorkBuddyAuth, error) {
	gate := GetGate()
	affinity := GetAffinity()
	quarantine := GetQuarantine()
	store := auth.GetStore()

	accounts := store.ListByRealm(realm, false)
	if len(accounts) == 0 {
		accounts = store.List()
	}
	if len(accounts) == 0 {
		return nil, nil, fmt.Errorf("no active accounts available in realm: %s", realm)
	}

	ordered := p.orderAccounts(accounts, preferredID, sessionKey, model, affinity, quarantine)
	var lastErr error

	for _, acc := range ordered {
		if !gate.TryAcquire(acc.ID) {
			continue
		}

		streamCh, err := p.exec.ExecuteStream(ctx, acc, req)
		if err == nil {
			if sessionKey != "" {
				affinity.BindSession(sessionKey, acc.ID)
			}

			// 包装通道以便在流结束时释放准入槽位
			wrappedCh := make(chan executor.StreamChunk, 32)
			go func(aID string, src <-chan executor.StreamChunk) {
				defer gate.Release(aID)
				defer close(wrappedCh)
				for chunk := range src {
					wrappedCh <- chunk
				}
			}(acc.ID, streamCh)

			return wrappedCh, acc, nil
		}

		gate.Release(acc.ID)
		lastErr = err
		p.handleError(acc, model, err, quarantine)
	}

	if lastErr != nil {
		return nil, nil, fmt.Errorf("all accounts in realm %s failed: %w", realm, lastErr)
	}
	return nil, nil, fmt.Errorf("all accounts in realm %s congested", realm)
}

func (p *Pipeline) orderAccounts(accounts []*auth.WorkBuddyAuth, preferredID, sessionKey, model string, affinity *AffinityManager, quarantine *QuarantineManager) []*auth.WorkBuddyAuth {
	targetID := preferredID
	if targetID == "" && sessionKey != "" {
		targetID = affinity.GetBoundAccount(sessionKey)
	}
	var priority, normal []*auth.WorkBuddyAuth

	for _, acc := range accounts {
		isQ, _ := quarantine.IsQuarantined(acc.ID, model)
		if isQ {
			continue
		}
		if targetID != "" && acc.ID == targetID {
			priority = append(priority, acc)
		} else {
			normal = append(normal, acc)
		}
	}

	return append(priority, normal...)
}

func (p *Pipeline) handleError(acc *auth.WorkBuddyAuth, model string, err error, quarantine *QuarantineManager) {
	upErr, ok := err.(*executor.UpstreamError)
	if !ok {
		return
	}

	if upErr.IsCreditExhausted() {
		// 积分耗尽，全局隔离该账号 30 分钟
		quarantine.QuarantineAccount(acc.ID, 30*time.Minute, "credits exhausted")
	} else if upErr.Is429() {
		// 频率限制，按模型隔离 60 秒
		quarantine.QuarantineModel(acc.ID, model, 60*time.Second, "rate limit (429)")
	} else if upErr.IsGatewayError() {
		// 网关抖动，短时间隔离 15 秒
		quarantine.QuarantineModel(acc.ID, model, 15*time.Second, "upstream gateway error")
	}
}
