package cpa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/internal/executor"
	"workbuddy-ai-cliproxy/internal/failover"
	"workbuddy-ai-cliproxy/internal/growth"
	"workbuddy-ai-cliproxy/internal/management"
	"workbuddy-ai-cliproxy/internal/models"
	"workbuddy-ai-cliproxy/internal/usage"
	"workbuddy-ai-cliproxy/pkg/version"
	"workbuddy-ai-cliproxy/web"

	"github.com/gin-gonic/gin"
)

// Dispatcher 负责 CLIProxyAPI (CPA) 全版本 JSON-RPC 请求的分发与执行
type Dispatcher struct {
	pipeline    *failover.Pipeline
	ginEngine   *gin.Engine
	loginStates sync.Map // state string -> *loginContext
	httpClient  *http.Client
}

type loginContext struct {
	client   *http.Client
	expires  time.Time
	realm    auth.Realm
	provider string
}

var (
	defaultDispatcher *Dispatcher
	onceDispatcher    sync.Once
)

// GetDispatcher 获取全局单例分发器
func GetDispatcher() *Dispatcher {
	onceDispatcher.Do(func() {
		gin.SetMode(gin.ReleaseMode)
		engine := gin.New()
		engine.Use(gin.Recovery())

		// 将管理路由挂载在根、CPA 官方规范及 workbuddyai 别名前缀上
		management.MountRoutes(engine.Group(""))
		management.MountRoutes(engine.Group("/v0/management/workbuddy"))
		management.MountRoutes(engine.Group("/v0/management/workbuddyai"))
		management.MountRoutes(engine.Group("/v0/management/plugins/workbuddy"))
		management.MountRoutes(engine.Group("/v0/management/plugins/workbuddyai"))
		management.MountRoutes(engine.Group("/v0/resource/plugins/workbuddy"))
		management.MountRoutes(engine.Group("/v0/resource/plugins/workbuddyai"))

		jar, _ := cookiejar.New(nil)
		client := &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		}

		defaultDispatcher = &Dispatcher{
			pipeline:   failover.NewPipeline(executor.NewWorkBuddyExecutor()),
			ginEngine:  engine,
			httpClient: client,
		}
	})
	return defaultDispatcher
}

// HandleMethod 处理指定 RPC 方法及载荷
func (d *Dispatcher) HandleMethod(ctx context.Context, method string, requestBytes []byte, bridge HostBridge) ([]byte, error) {
	if bridge == nil {
		bridge = &NoopHostBridge{}
	}

	switch method {
	case MethodPluginRegister, MethodPluginReconfigure:
		return d.handleRegister(requestBytes)
	case MethodPluginQuiesce:
		return okEnvelope(true)
	case MethodPluginShutdown:
		growth.StopScheduler()
		return okEnvelope(true)

	case MethodModelStatic, MethodModelForAuth, MethodModelRegister:
		return d.handleModels()

	case MethodAuthIdentifier, MethodExecutorIdentifier, MethodQuotaIdentifier:
		// 导出 workbuddyai，支持在 CPAMP 及 CPA 中展示为标准的 WorkBuddy AI OAuth Provider
		return okEnvelope(IdentifierResponse{Identifier: "workbuddyai"})

	case MethodAuthParse:
		return d.handleAuthParse(requestBytes)
	case MethodAuthLoginStart:
		return d.handleAuthLoginStart(requestBytes)
	case MethodAuthLoginPoll:
		return d.handleAuthLoginPoll(requestBytes)
	case MethodAuthRefresh:
		return d.handleAuthRefresh(ctx, requestBytes)

	case MethodExecutorExecute:
		return d.handleExecutorExecute(ctx, requestBytes)
	case MethodExecutorExecuteStream:
		return d.handleExecutorExecuteStream(ctx, requestBytes, bridge)

	case MethodManagementRegister:
		return d.handleManagementRegister()
	case MethodManagementHandle:
		return d.handleManagementHandle(requestBytes)

	case MethodQuotaDescribe:
		return d.handleQuotaDescribe()
	case MethodQuotaFetch:
		return d.handleQuotaFetch(ctx, requestBytes)

	default:
		return errorEnvelope("unknown_method", fmt.Sprintf("unsupported method: %s", method)), nil
	}
}

// -----------------------------------------------------------------------------
// Plugin Registration & Lifecycle
// -----------------------------------------------------------------------------

func (d *Dispatcher) handleRegister(reqBytes []byte) ([]byte, error) {
	var req LifecycleRequest
	if len(reqBytes) > 0 {
		_ = json.Unmarshal(reqBytes, &req)
	}

	if len(req.ConfigYAML) > 0 {
		var rawMap map[string]any
		if err := json.Unmarshal(req.ConfigYAML, &rawMap); err == nil {
			_ = config.LoadFromMap(rawMap)
		}
	}

	// 动态协商 schema_version: 不超过宿主要求，也不超过我们支持的最高版本
	schemaVer := req.SchemaVersion
	if schemaVer > MaxSupportedSchemaVersion {
		schemaVer = MaxSupportedSchemaVersion
	}
	if schemaVer == 0 {
		schemaVer = 1
	}

	resp := Registration{
		SchemaVersion: schemaVer,
		Metadata: Metadata{
			Name:             "WorkBuddy AI",
			Version:          version.Version,
			Author:           version.Author,
			GitHubRepository: "https://github.com/lihongxu0221/workbuddy-ai-cliproxy",
		},
		Capabilities: Capabilities{
			ModelProvider:         true,
			AuthProvider:          true,
			Executor:              true,
			ExecutorModelScope:    "both",
			ExecutorInputFormats:  []string{"chat-completions"},
			ExecutorOutputFormats: []string{"chat-completions"},
			ManagementAPI:         true,
			QuotaProvider:         true,
		},
	}

	return okEnvelope(resp)
}

// -----------------------------------------------------------------------------
// Models
// -----------------------------------------------------------------------------

func (d *Dispatcher) handleModels() ([]byte, error) {
	store := auth.GetStore()
	accounts := store.List()

	reg := models.GetRegistry()
	all := reg.ListUsable(accounts)

	var result []ModelInfo
	for _, m := range all {
		if m.Disabled {
			continue
		}
		contextLen := int64(m.ContextLength)
		if contextLen == 0 {
			contextLen = 128000
		}

		// 1. 注册原生未带前缀模型
		result = append(result, ModelInfo{
			ID:                         m.ID,
			Object:                     "model",
			OwnedBy:                    "workbuddyai",
			DisplayName:                m.DisplayName,
			Name:                       m.ID,
			SupportedGenerationMethods: []string{"chat"},
			ContextLength:              contextLen,
			MaxCompletionTokens:        8192,
			UserDefined:                true,
		})

		// 2. 依据 Realm 自动添加带路由前缀模型
		if m.Realm == auth.RealmCN {
			result = append(result, ModelInfo{
				ID:                         "cn/" + m.ID,
				Object:                     "model",
				OwnedBy:                    "workbuddyai",
				DisplayName:                fmt.Sprintf("[CN] %s", m.DisplayName),
				Name:                       "cn/" + m.ID,
				SupportedGenerationMethods: []string{"chat"},
				ContextLength:              contextLen,
				MaxCompletionTokens:        8192,
				UserDefined:                true,
			})
		} else {
			// Global / en 前缀: 支持 cb/ 与 en/
			result = append(result, ModelInfo{
				ID:                         "cb/" + m.ID,
				Object:                     "model",
				OwnedBy:                    "workbuddyai",
				DisplayName:                fmt.Sprintf("[Global] %s", m.DisplayName),
				Name:                       "cb/" + m.ID,
				SupportedGenerationMethods: []string{"chat"},
				ContextLength:              contextLen,
				MaxCompletionTokens:        8192,
				UserDefined:                true,
			})
			result = append(result, ModelInfo{
				ID:                         "en/" + m.ID,
				Object:                     "model",
				OwnedBy:                    "workbuddyai",
				DisplayName:                fmt.Sprintf("[EN] %s", m.DisplayName),
				Name:                       "en/" + m.ID,
				SupportedGenerationMethods: []string{"chat"},
				ContextLength:              contextLen,
				MaxCompletionTokens:        8192,
				UserDefined:                true,
			})
		}
	}

	return okEnvelope(ModelResponse{
		Provider: "workbuddyai",
		Models:   result,
	})
}

// -----------------------------------------------------------------------------
// Auth Parsing, Login & Refresh
// -----------------------------------------------------------------------------

func (d *Dispatcher) handleAuthParse(raw []byte) ([]byte, error) {
	var req AuthParseRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	accounts, err := auth.ParseAuthPayload(req.RawJSON, req.FileName, req.Path)
	if err != nil || len(accounts) == 0 {
		return okEnvelope(AuthParseResponse{Handled: false})
	}

	store := auth.GetStore()
	var authDatas []AuthData

	for _, acc := range accounts {
		store.UpsertInMemory(acc)
		authDatas = append(authDatas, toAuthData(acc, req.FileName))
	}

	return okEnvelope(AuthParseResponse{
		Handled: true,
		Auth:    authDatas[0],
		Auths:   authDatas,
	})
}

func toAuthData(acc *auth.WorkBuddyAuth, fileName string) AuthData {
	storageJSON, _ := json.Marshal(acc)
	if fileName == "" {
		fileName = fmt.Sprintf("%s.json", acc.ID)
	}

	label := fmt.Sprintf("WorkBuddy [%s]", strings.ToUpper(string(acc.Realm)))
	if acc.Nickname != "" {
		label += fmt.Sprintf(" (%s)", acc.Nickname)
	} else if acc.UserID != "" {
		label += fmt.Sprintf(" (UID:%s)", acc.UserID)
	}

	prefix := "cb/"
	if acc.Realm == auth.RealmCN {
		prefix = "cn/"
	}

	var nextRefresh time.Time
	if acc.ExpiresAt > 0 {
		nextRefresh = time.Unix(acc.ExpiresAt-600, 0)
	}

	return AuthData{
		Provider:         "workbuddyai",
		ID:               acc.ID,
		FileName:         fileName,
		Label:            label,
		Prefix:           prefix,
		ProxyURL:         acc.ProxyURL,
		Disabled:         acc.Disabled,
		StorageJSON:      storageJSON,
		Metadata:         map[string]any{"type": "workbuddyai", "realm": acc.Realm, "user_id": acc.UserID},
		Attributes:       map[string]string{"realm": string(acc.Realm)},
		NextRefreshAfter: nextRefresh,
	}
}

func (d *Dispatcher) handleAuthLoginStart(raw []byte) ([]byte, error) {
	var req AuthLoginStartRequest
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}

	loginClient := config.NewHTTPClient(30*time.Second, "")
	jar, _ := cookiejar.New(nil)
	loginClient.Jar = jar

	isCN := false
	if req.Provider == "workbuddy-cn" || req.Provider == "codebuddy" {
		isCN = true
	}
	if req.Metadata != nil {
		if r, ok := req.Metadata["realm"].(string); ok && strings.ToLower(r) == "cn" {
			isCN = true
		}
	}

	endpoint := "https://www.codebuddy.ai/v2/plugin/auth/state?platform=CLI"
	origin := "https://www.codebuddy.ai"
	providerID := "workbuddyai"
	realm := auth.RealmGlobal

	if isCN {
		endpoint = "https://copilot.tencent.com/v2/plugin/auth/state?platform=CLI"
		origin = "https://www.codebuddy.cn"
		providerID = "workbuddy"
		realm = auth.RealmCN
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	httpReq.Header.Set("Origin", origin)
	httpReq.Header.Set("Referer", origin+"/")

	resp, err := loginClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("start login failed: %w", err)
	}
	defer resp.Body.Close()

	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			State   string `json:"state"`
			AuthURL string `json:"authUrl"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decode auth state failed: %w", err)
	}

	if env.Code != 0 || env.Data.State == "" || env.Data.AuthURL == "" {
		return nil, fmt.Errorf("auth state error code %d: %s", env.Code, env.Msg)
	}

	ttl := 5 * time.Minute
	d.loginStates.Store(env.Data.State, &loginContext{
		client:   loginClient,
		expires:  time.Now().Add(ttl),
		realm:    realm,
		provider: providerID,
	})

	return okEnvelope(AuthLoginStartResponse{
		Provider:  providerID,
		URL:       env.Data.AuthURL,
		State:     env.Data.State,
		ExpiresAt: time.Now().Add(ttl).UTC(),
	})
}

func (d *Dispatcher) handleAuthLoginPoll(raw []byte) ([]byte, error) {
	var req AuthLoginPollRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	state := strings.TrimSpace(req.State)
	if state == "" {
		return nil, fmt.Errorf("empty login state")
	}

	val, ok := d.loginStates.Load(state)
	if !ok {
		return nil, fmt.Errorf("login state not found or expired")
	}
	lc := val.(*loginContext)
	if time.Now().After(lc.expires) {
		d.loginStates.Delete(state)
		return nil, fmt.Errorf("login state expired")
	}

	baseURL := "https://www.codebuddy.ai"
	origin := "https://www.codebuddy.ai"
	if lc.realm == auth.RealmCN {
		baseURL = "https://copilot.tencent.com"
		origin = "https://www.codebuddy.cn"
	}

	tokenURL := fmt.Sprintf("%s/v2/plugin/auth/token?state=%s", baseURL, state)
	httpReq, err := http.NewRequest(http.MethodGet, tokenURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	httpReq.Header.Set("Origin", origin)
	httpReq.Header.Set("Referer", origin+"/")

	resp, err := lc.client.Do(httpReq)
	if err != nil {
		return okEnvelope(AuthLoginPollResponse{
			Provider: lc.provider,
			Status:   "pending",
			Message:  "waiting for login",
		})
	}
	defer resp.Body.Close()

	var tokenEnv struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int64  `json:"expiresIn"`
			Domain       string `json:"domain"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenEnv); err != nil || tokenEnv.Data.AccessToken == "" {
		return okEnvelope(AuthLoginPollResponse{
			Provider: lc.provider,
			Status:   "pending",
			Message:  "waiting for login confirmation",
		})
	}

	// 拉取账号元数据
	acctURL := fmt.Sprintf("%s/v2/plugin/login/account?state=%s", baseURL, state)
	acctReq, _ := http.NewRequest(http.MethodGet, acctURL, nil)
	acctReq.Header.Set("Authorization", "Bearer "+tokenEnv.Data.AccessToken)
	acctReq.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	acctReq.Header.Set("Origin", origin)

	var uid, nickname, enterpriseID string
	if acctResp, errAcct := lc.client.Do(acctReq); errAcct == nil {
		defer acctResp.Body.Close()
		var acctEnv struct {
			Data struct {
				UID          string `json:"uid"`
				EnterpriseID string `json:"enterpriseId"`
				Nickname     string `json:"nickname"`
			} `json:"data"`
		}
		_ = json.NewDecoder(acctResp.Body).Decode(&acctEnv)
		uid = acctEnv.Data.UID
		nickname = acctEnv.Data.Nickname
		enterpriseID = acctEnv.Data.EnterpriseID
	}

	accID := fmt.Sprintf("workbuddy-%s-%s", uid, lc.realm)
	if uid == "" {
		accID = fmt.Sprintf("workbuddy-%s-%d", lc.realm, time.Now().Unix()%100000)
	}

	acc := &auth.WorkBuddyAuth{
		Type:         "workbuddy",
		ID:           accID,
		Realm:        lc.realm,
		AccessToken:  tokenEnv.Data.AccessToken,
		RefreshToken: tokenEnv.Data.RefreshToken,
		ExpiresAt:    time.Now().Unix() + tokenEnv.Data.ExpiresIn,
		UserID:       uid,
		EnterpriseID: enterpriseID,
		Nickname:     nickname,
	}

	// 尝试拉取一次初始余额
	checker := auth.NewBalanceChecker()
	credits, pkgs, _ := checker.FetchBalance(context.Background(), acc)
	acc.Credits = credits
	acc.CreditPackages = pkgs

	_ = auth.GetStore().Save(acc)
	d.loginStates.Delete(state)

	return okEnvelope(AuthLoginPollResponse{
		Provider: lc.provider,
		Status:   "success",
		Auth:     toAuthData(acc, fmt.Sprintf("%s.json", acc.ID)),
	})
}

func (d *Dispatcher) handleAuthRefresh(ctx context.Context, raw []byte) ([]byte, error) {
	var req AuthRefreshRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	parsedList, err := auth.ParseAuthPayload(req.StorageJSON, "", "")
	if err != nil || len(parsedList) == 0 {
		return nil, fmt.Errorf("invalid storage json in refresh: %w", err)
	}

	acc := parsedList[0]
	refresher := auth.NewRefresher()
	refreshed, err := refresher.ForceRefresh(ctx, acc)
	if err != nil {
		return nil, fmt.Errorf("force refresh failed: %w", err)
	}

	return okEnvelope(AuthRefreshResponse{
		Auth: toAuthData(refreshed, ""),
	})
}

// -----------------------------------------------------------------------------
// Executor
// -----------------------------------------------------------------------------

func (d *Dispatcher) handleExecutorExecute(ctx context.Context, raw []byte) ([]byte, error) {
	var req ExecutorRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	parsedModel := models.ParsePrefix(req.Model)
	var preferredID string
	targetRealm := parsedModel.TargetRealm

	if len(req.StorageJSON) > 0 {
		if accs, err := auth.ParseAuthPayload(req.StorageJSON, "", ""); err == nil && len(accs) > 0 {
			preferredID = accs[0].ID
			targetRealm = accs[0].Realm
		}
	}

	payload := req.Payload
	if len(payload) == 0 {
		payload = req.OriginalRequest
	}

	start := time.Now()
	resp, err := d.pipeline.ExecuteWithFailover(ctx, preferredID, targetRealm, parsedModel.CleanModel, preferredID, executor.Request{
		Model:   parsedModel.CleanModel,
		Payload: payload,
		Stream:  false,
		Header:  req.Headers,
	})

	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		usage.GetLogger().LogRequest(preferredID, string(targetRealm), parsedModel.CleanModel, nil, durationMs, false, err.Error())
		return errorEnvelope("execution_error", err.Error(), 502), nil
	}

	usage.GetLogger().LogRequest(preferredID, string(targetRealm), parsedModel.CleanModel, resp.Usage, durationMs, true, "")

	return okEnvelope(ExecutorResponse{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Payload: resp.Payload,
	})
}

func (d *Dispatcher) handleExecutorExecuteStream(ctx context.Context, raw []byte, bridge HostBridge) ([]byte, error) {
	var req ExecutorStreamRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	parsedModel := models.ParsePrefix(req.Model)
	var preferredID string
	targetRealm := parsedModel.TargetRealm

	if len(req.StorageJSON) > 0 {
		if accs, err := auth.ParseAuthPayload(req.StorageJSON, "", ""); err == nil && len(accs) > 0 {
			preferredID = accs[0].ID
			targetRealm = accs[0].Realm
		}
	}

	payload := req.Payload
	if len(payload) == 0 {
		payload = req.OriginalRequest
	}

	streamHeaders := http.Header{
		"Content-Type":      []string{"text/event-stream"},
		"Cache-Control":     []string{"no-cache"},
		"X-Accel-Buffering": []string{"no"},
	}

	sseFramed := executor.ClientNeedsSSEFrame(req.Metadata)

	streamCh, actualAcc, err := d.pipeline.ExecuteStreamWithFailover(ctx, preferredID, targetRealm, parsedModel.CleanModel, preferredID, executor.Request{
		Model:   parsedModel.CleanModel,
		Payload: payload,
		Stream:  true,
		Header:  req.Headers,
	})

	if err != nil {
		return errorEnvelope("stream_error", err.Error(), 502), nil
	}

	// 1. 若宿主提供了 stream_id 且非独立模式，使用 Host.Stream.Emit 进行真异步流式推流
	if req.StreamID != "" {
		go func(sID string, ch <-chan executor.StreamChunk, accID string, realm string, m string) {
			defer bridge.StreamClose(sID)
			start := time.Now()
			var lastUsage *executor.UsageInfo

			for chunk := range ch {
				if chunk.Err != nil {
					break
				}
				if chunk.Usage != nil {
					lastUsage = chunk.Usage
				}
				content := executor.StripDataPrefix(string(chunk.Payload))
				if content == "" || content == "[DONE]" {
					continue
				}
				cleaned := executor.CleanChunkJSON(content)
				if cleaned == "" {
					continue
				}
				if sseFramed {
					cleaned = "data: " + cleaned
				}
				if err := bridge.StreamEmit(sID, []byte(cleaned)); err != nil {
					break
				}
			}
			durationMs := time.Since(start).Milliseconds()
			usage.GetLogger().LogRequest(accID, realm, m, lastUsage, durationMs, true, "")
		}(req.StreamID, streamCh, actualAcc.ID, string(actualAcc.Realm), parsedModel.CleanModel)

		return okEnvelope(StreamResponse{
			Headers: streamHeaders,
		})
	}

	// 2. 否则同步排空流式块并以切片形式直接返回 (兼容同步流模式)
	var chunks []ExecutorStreamChunk
	for chunk := range streamCh {
		if chunk.Err != nil {
			break
		}
		content := executor.StripDataPrefix(string(chunk.Payload))
		if content == "" || content == "[DONE]" {
			continue
		}
		cleaned := executor.CleanChunkJSON(content)
		if cleaned == "" {
			continue
		}
		if sseFramed {
			cleaned = "data: " + cleaned
		}
		chunks = append(chunks, ExecutorStreamChunk{
			Payload: json.RawMessage([]byte(cleaned)),
		})
	}

	return okEnvelope(StreamResponse{
		Headers: streamHeaders,
		Chunks:  chunks,
	})
}

// -----------------------------------------------------------------------------
// Management API
// -----------------------------------------------------------------------------

func (d *Dispatcher) handleManagementRegister() ([]byte, error) {
	// 同时注册 /workbuddyai/... 与 /workbuddy/... 路由，无缝对接 CPA 与 CPAMP (避免 head 为 "plugins" 触发 CPAMP admin key 校验)
	routes := []ManagementRoute{
		{Method: "GET", Path: "/workbuddyai/accounts", Description: "获取所有账号及状态"},
		{Method: "POST", Path: "/workbuddyai/accounts/import", Description: "导入账号凭据"},
		{Method: "POST", Path: "/workbuddyai/accounts/refresh", Description: "刷新账号 Token 与余额"},
		{Method: "DELETE", Path: "/workbuddyai/accounts/:id", Description: "删除账号"},
		{Method: "POST", Path: "/workbuddyai/actions/dailybonus", Description: "触发国际区 30 积分打卡"},
		{Method: "POST", Path: "/workbuddyai/actions/checkin", Description: "触发国内区 100 积分签到"},
		{Method: "POST", Path: "/workbuddyai/actions/trial", Description: "激活国际区 250 积分 Expert 权益"},
		{Method: "GET", Path: "/workbuddyai/dailybonus/status", Description: "获取今日打卡热力图"},
		{Method: "POST", Path: "/workbuddyai/actions/login-start", Description: "启动扫码或OAuth登录"},
		{Method: "GET", Path: "/workbuddyai/actions/login-poll", Description: "轮询扫码或OAuth状态"},
		{Method: "POST", Path: "/workbuddyai/actions/login-poll", Description: "轮询扫码或OAuth状态"},
		{Method: "GET", Path: "/workbuddyai/models", Description: "获取全部模型列表"},
		{Method: "POST", Path: "/workbuddyai/models/toggle", Description: "开关模型"},
		{Method: "POST", Path: "/workbuddyai/models/test", Description: "测试模型连通性与扣费"},
		{Method: "GET", Path: "/workbuddyai/stats", Description: "获取统计概览"},
		{Method: "GET", Path: "/workbuddyai/config", Description: "获取配置"},
		{Method: "POST", Path: "/workbuddyai/config", Description: "更新配置"},

		// workbuddy 别名兼容
		{Method: "GET", Path: "/workbuddy/accounts", Description: "获取所有账号及状态"},
		{Method: "POST", Path: "/workbuddy/accounts/import", Description: "导入账号凭据"},
		{Method: "POST", Path: "/workbuddy/accounts/refresh", Description: "刷新账号 Token 与余额"},
		{Method: "DELETE", Path: "/workbuddy/accounts/:id", Description: "删除账号"},
		{Method: "POST", Path: "/workbuddy/actions/dailybonus", Description: "触发国际区 30 积分打卡"},
		{Method: "POST", Path: "/workbuddy/actions/checkin", Description: "触发国内区 100 积分签到"},
		{Method: "POST", Path: "/workbuddy/actions/trial", Description: "激活国际区 250 积分 Expert 权益"},
		{Method: "GET", Path: "/workbuddy/dailybonus/status", Description: "获取今日打卡热力图"},
		{Method: "POST", Path: "/workbuddy/actions/login-start", Description: "启动扫码或OAuth登录"},
		{Method: "GET", Path: "/workbuddy/actions/login-poll", Description: "轮询扫码或OAuth状态"},
		{Method: "POST", Path: "/workbuddy/actions/login-poll", Description: "轮询扫码或OAuth状态"},
		{Method: "GET", Path: "/workbuddy/models", Description: "获取全部模型列表"},
		{Method: "POST", Path: "/workbuddy/models/toggle", Description: "开关模型"},
		{Method: "POST", Path: "/workbuddy/models/test", Description: "测试模型连通性与扣费"},
		{Method: "GET", Path: "/workbuddy/stats", Description: "获取统计概览"},
		{Method: "GET", Path: "/workbuddy/config", Description: "获取配置"},
		{Method: "POST", Path: "/workbuddy/config", Description: "更新配置"},

		// 官方 /plugins/... 备用
		{Method: "GET", Path: "/plugins/workbuddyai/accounts", Description: "获取所有账号及状态"},
		{Method: "POST", Path: "/plugins/workbuddyai/actions/login-start", Description: "启动扫码或OAuth登录"},
		{Method: "GET", Path: "/plugins/workbuddyai/actions/login-poll", Description: "轮询扫码或OAuth状态"},
		{Method: "GET", Path: "/plugins/workbuddy/accounts", Description: "获取所有账号及状态"},
		{Method: "POST", Path: "/plugins/workbuddy/actions/login-start", Description: "启动扫码或OAuth登录"},
		{Method: "GET", Path: "/plugins/workbuddy/actions/login-poll", Description: "轮询扫码或OAuth状态"},
	}

	resources := []ResourceRoute{
		{
			Path:        "/ui",
			Menu:        "WorkBuddy 控制台",
			Description: "WorkBuddy 账号池、多区域模型与积分监控控制台",
		},
	}

	return okEnvelope(ManagementRegistrationResponse{
		Routes:    routes,
		Resources: resources,
	})
}

func (d *Dispatcher) handleManagementHandle(raw []byte) ([]byte, error) {
	var req ManagementRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	path := strings.TrimSpace(req.Path)

	// 核心修复：如果是 Web 控制台页面请求，直接返回注入了 ManagementKey 的自包含 HTML，彻底避免 301 重定向与鉴权遗漏
	if path == "" ||
		strings.HasSuffix(path, "/ui") ||
		strings.HasSuffix(path, "/ui/") ||
		strings.HasSuffix(path, "/index.html") ||
		path == "/v0/resource/plugins/workbuddy/ui" ||
		path == "/v0/resource/plugins/workbuddyai/ui" {
		return okEnvelope(ManagementResponse{
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Content-Type":  []string{"text/html; charset=utf-8"},
				"Cache-Control": []string{"no-cache, no-store, must-revalidate"},
			},
			Body: web.RenderFullHTML(config.GetManagementKey()),
		})
	}

	// 针对 API 请求构建 http.Request 派发给 Gin 引擎
	targetPath := path
	if req.Query != nil && len(req.Query) > 0 {
		targetPath += "?" + req.Query.Encode()
	}

	httpReq, err := http.NewRequest(req.Method, targetPath, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	httpReq.Header = req.Headers

	recorder := httptest.NewRecorder()
	d.ginEngine.ServeHTTP(recorder, httpReq)

	return okEnvelope(ManagementResponse{
		StatusCode: recorder.Code,
		Headers:    recorder.Header(),
		Body:       recorder.Body.Bytes(),
	})
}

// -----------------------------------------------------------------------------
// Quota Provider
// -----------------------------------------------------------------------------

func (d *Dispatcher) handleQuotaDescribe() ([]byte, error) {
	return okEnvelope(QuotaDescribeResponse{
		SupportedProviders: []string{"workbuddyai", "workbuddy", "codebuddy"},
		DisplayName:        "WorkBuddy AI / CodeBuddy",
		SupportsReset:      false,
	})
}

func (d *Dispatcher) handleQuotaFetch(ctx context.Context, raw []byte) ([]byte, error) {
	var req QuotaFetchRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	store := auth.GetStore()
	var targetAcc *auth.WorkBuddyAuth

	if req.AuthID != "" {
		targetAcc = store.GetByID(req.AuthID)
	}

	if targetAcc == nil && len(req.StorageJSON) > 0 {
		if accs, err := auth.ParseAuthPayload(req.StorageJSON, "", ""); err == nil && len(accs) > 0 {
			targetAcc = accs[0]
		}
	}

	if targetAcc == nil {
		return errorEnvelope("auth_not_found", "account not found for quota fetch", 404), nil
	}

	checker := auth.NewBalanceChecker()
	credits, pkgs, err := checker.FetchBalance(ctx, targetAcc)
	if err != nil {
		credits = targetAcc.Credits
		pkgs = targetAcc.CreditPackages
	}

	return okEnvelope(QuotaFetchResponse{
		Provider:       "workbuddyai",
		RemainingQuota: credits,
		TotalQuota:     1000.0,
		IsUnlimited:    false,
		Currency:       "CREDITS",
		QuotaType:      string(targetAcc.Realm),
		Details: map[string]any{
			"realm":           targetAcc.Realm,
			"user_id":         targetAcc.UserID,
			"packages":        pkgs,
			"remaining_coins": credits,
		},
	})
}

// -----------------------------------------------------------------------------
// Envelope Helpers
// -----------------------------------------------------------------------------

func okEnvelope(v any) ([]byte, error) {
	result, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{OK: true, Result: result})
}

func errorEnvelope(code, message string, httpStatus ...int) []byte {
	status := 500
	if len(httpStatus) > 0 && httpStatus[0] > 0 {
		status = httpStatus[0]
	}
	raw, _ := json.Marshal(Envelope{
		OK: false,
		Error: &Error{
			Code:       code,
			Message:    message,
			HTTPStatus: status,
		},
	})
	return raw
}
