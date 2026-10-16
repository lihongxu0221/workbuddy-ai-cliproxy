package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"workbuddy-ai-cliproxy/internal/auth"
)

// Request 下游转译好的请求载荷
type Request struct {
	Model   string
	Payload []byte
	Stream  bool
	Header  http.Header
}

// Response 非流式响应载荷
type Response struct {
	Payload []byte
	Usage   *UsageInfo
}

// WorkBuddyExecutor 核心执行器，实现 CPA ProviderExecutor 接口规范
type WorkBuddyExecutor struct {
	client    *UpstreamClient
	refresher *auth.Refresher
}

// NewWorkBuddyExecutor 实例化执行器
func NewWorkBuddyExecutor() *WorkBuddyExecutor {
	return &WorkBuddyExecutor{
		client:    NewUpstreamClient(),
		refresher: auth.NewRefresher(),
	}
}

// Identifier 返回此执行器在 CPA 内部挂载的 Provider 唯一标识
func (e *WorkBuddyExecutor) Identifier() string {
	return "workbuddy"
}

// PrepareRequest 为原生 HTTP 请求注入鉴权标头
func (e *WorkBuddyExecutor) PrepareRequest(req *http.Request, a *auth.WorkBuddyAuth) error {
	ApplyStandardHeaders(req, a)
	return nil
}

// Execute 执行非流式请求 (针对腾讯 11101 错误，强制上游流式并在内存中聚合)
func (e *WorkBuddyExecutor) Execute(ctx context.Context, a *auth.WorkBuddyAuth, req Request) (Response, error) {
	acc, err := e.refresher.RefreshIfExpired(ctx, a)
	if err != nil {
		acc = a
	}

	body := RewriteSystemForUpstream(ForceStreamBody(req.Payload, nil))
	apiURL := fmt.Sprintf("%s/v2/chat/completions", acc.BaseURL())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create upstream request failed: %w", err)
	}

	ApplyStandardHeaders(httpReq, acc)
	httpClient := e.client.ClientForAuth(acc)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("do upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return Response{Payload: bodyBytes}, &UpstreamError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	completionBytes, err := AggregateCompletion(resp.Body, req.Model)
	if err != nil {
		return Response{}, fmt.Errorf("aggregate completion failed: %w", err)
	}

	usage := ParseTerminalUsage(completionBytes)
	return Response{
		Payload: completionBytes,
		Usage:   usage,
	}, nil
}

// ExecuteStream 执行流式请求并返回 SSE 块通道
func (e *WorkBuddyExecutor) ExecuteStream(ctx context.Context, a *auth.WorkBuddyAuth, req Request) (<-chan StreamChunk, error) {
	acc, err := e.refresher.RefreshIfExpired(ctx, a)
	if err != nil {
		acc = a
	}

	body := RewriteSystemForUpstream(ForceStreamBody(req.Payload, nil))
	apiURL := fmt.Sprintf("%s/v2/chat/completions", acc.BaseURL())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create stream request failed: %w", err)
	}

	ApplyStandardHeaders(httpReq, acc)
	httpClient := e.client.ClientForAuth(acc)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do stream request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	return ReadSSEStream(ctx, resp.Body), nil
}

// Refresh 刷新账号凭据
func (e *WorkBuddyExecutor) Refresh(ctx context.Context, a *auth.WorkBuddyAuth) (*auth.WorkBuddyAuth, error) {
	return e.refresher.ForceRefresh(ctx, a)
}

// UpstreamError 统一上游异常封装
type UpstreamError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream error (HTTP %d): %s", e.StatusCode, e.Body)
}

// Is429 是否为频率限制
func (e *UpstreamError) Is429() bool {
	return e.StatusCode == http.StatusTooManyRequests ||
		strings.Contains(e.Body, "11200") ||
		strings.Contains(e.Body, "rate limit")
}

// IsCreditExhausted 是否为积分耗尽 (402 或 code 14018)
func (e *UpstreamError) IsCreditExhausted() bool {
	return e.StatusCode == http.StatusPaymentRequired ||
		strings.Contains(e.Body, "14018") ||
		strings.Contains(e.Body, "insufficient credit") ||
		strings.Contains(e.Body, "credit balance")
}

// IsGatewayError 是否为 6004 或 502/504 网关异常
func (e *UpstreamError) IsGatewayError() bool {
	return e.StatusCode >= 500 || strings.Contains(e.Body, "6004")
}
