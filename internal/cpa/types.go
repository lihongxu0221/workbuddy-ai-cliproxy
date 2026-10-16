package cpa

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// ABIVersion tracks the native C ABI shape
const ABIVersion uint32 = 1

// MaxSupportedSchemaVersion tracks the maximum supported RPC schema version
const MaxSupportedSchemaVersion uint32 = 6

// RPC Method constants
const (
	MethodPluginRegister    = "plugin.register"
	MethodPluginQuiesce     = "plugin.quiesce"
	MethodPluginReconfigure = "plugin.reconfigure"
	MethodPluginShutdown    = "plugin.shutdown"

	MethodModelRegister = "model.register"
	MethodModelStatic   = "model.static"
	MethodModelForAuth  = "model.for_auth"

	MethodAuthIdentifier = "auth.identifier"
	MethodAuthParse      = "auth.parse"
	MethodAuthLoginStart = "auth.login.start"
	MethodAuthLoginPoll  = "auth.login.poll"
	MethodAuthRefresh    = "auth.refresh"

	MethodExecutorIdentifier    = "executor.identifier"
	MethodExecutorExecute       = "executor.execute"
	MethodExecutorExecuteStream = "executor.execute_stream"

	MethodManagementRegister = "management.register"
	MethodManagementHandle   = "management.handle"

	MethodQuotaIdentifier = "quota.identifier"
	MethodQuotaDescribe   = "quota.describe"
	MethodQuotaFetch      = "quota.fetch"

	MethodHostStreamEmit  = "host.stream.emit"
	MethodHostStreamClose = "host.stream.close"
)

// Envelope 统一 RPC 响应信封
type Envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Error RPC 错误描述
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// LifecycleRequest 宿主生命周期请求
type LifecycleRequest struct {
	ConfigYAML    []byte `json:"config_yaml"`
	SchemaVersion uint32 `json:"schema_version"`
}

// Metadata 插件信息
type Metadata struct {
	Name             string `json:"Name"`
	Version          string `json:"Version"`
	Author           string `json:"Author"`
	GitHubRepository string `json:"GitHubRepository,omitempty"`
	Logo             string `json:"Logo,omitempty"`
}

// Capabilities 插件能力声明
type Capabilities struct {
	ModelProvider         bool     `json:"model_provider"`
	AuthProvider          bool     `json:"auth_provider"`
	Executor              bool     `json:"executor"`
	ExecutorModelScope    string   `json:"executor_model_scope,omitempty"` // "both"
	ExecutorInputFormats  []string `json:"executor_input_formats,omitempty"`
	ExecutorOutputFormats []string `json:"executor_output_formats,omitempty"`
	ManagementAPI         bool     `json:"management_api"`
	QuotaProvider         bool     `json:"quota_provider"`
}

// Registration 插件注册响应
type Registration struct {
	SchemaVersion uint32       `json:"schema_version"`
	Metadata      Metadata     `json:"metadata"`
	Capabilities  Capabilities `json:"capabilities"`
}

// IdentifierResponse 返回 provider identifier
type IdentifierResponse struct {
	Identifier string `json:"identifier"`
}

// ModelInfo 模型定义
type ModelInfo struct {
	ID                         string   `json:"id"`
	Object                     string   `json:"object"`
	OwnedBy                    string   `json:"owned_by"`
	DisplayName                string   `json:"display_name"`
	Name                       string   `json:"name"`
	SupportedGenerationMethods []string `json:"supported_generation_methods"`
	ContextLength              int64    `json:"context_length"`
	MaxCompletionTokens        int64    `json:"max_completion_tokens"`
	UserDefined                bool     `json:"user_defined"`
}

// ModelResponse 模型列表响应
type ModelResponse struct {
	Provider string      `json:"provider"`
	Models   []ModelInfo `json:"models"`
}

// AuthData 凭据元数据
type AuthData struct {
	Provider         string            `json:"provider"`
	ID               string            `json:"id"`
	FileName         string            `json:"file_name"`
	Label            string            `json:"label"`
	Prefix           string            `json:"prefix,omitempty"`
	ProxyURL         string            `json:"proxy_url,omitempty"`
	Disabled         bool              `json:"disabled"`
	StorageJSON      json.RawMessage   `json:"storage_json"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	Attributes       map[string]string `json:"attributes,omitempty"`
	NextRefreshAfter time.Time         `json:"next_refresh_after,omitempty"`
}

// AuthParseRequest 解析凭据请求
type AuthParseRequest struct {
	Provider string          `json:"provider"`
	Path     string          `json:"path"`
	FileName string          `json:"file_name"`
	RawJSON  json.RawMessage `json:"raw_json"`
}

// AuthParseResponse 凭据解析响应
type AuthParseResponse struct {
	Handled bool       `json:"handled"`
	Auth    AuthData   `json:"auth"`
	Auths   []AuthData `json:"auths,omitempty"`
}

// AuthLoginStartRequest 开始登录请求
type AuthLoginStartRequest struct {
	Provider string         `json:"provider"`
	BaseURL  string         `json:"base_url"`
	Metadata map[string]any `json:"metadata"`
}

// AuthLoginStartResponse 开始登录响应
type AuthLoginStartResponse struct {
	Provider  string         `json:"provider"`
	URL       string         `json:"url"`
	State     string         `json:"state"`
	ExpiresAt time.Time      `json:"expires_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// AuthLoginPollRequest 轮询登录请求
type AuthLoginPollRequest struct {
	Provider string         `json:"provider"`
	State    string         `json:"state"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AuthLoginPollResponse 轮询登录响应
type AuthLoginPollResponse struct {
	Provider string   `json:"provider,omitempty"`
	Status   string   `json:"status"` // "pending" | "success" | "error"
	Message  string   `json:"message,omitempty"`
	Auth     AuthData `json:"auth,omitempty"`
}

// AuthRefreshRequest 刷新凭据请求
type AuthRefreshRequest struct {
	Provider    string          `json:"provider"`
	StorageJSON json.RawMessage `json:"storage_json"`
}

// AuthRefreshResponse 刷新凭据响应
type AuthRefreshResponse struct {
	Auth AuthData `json:"auth"`
}

// ExecutorRequest 执行器调用请求
type ExecutorRequest struct {
	Model           string          `json:"model"`
	Payload         json.RawMessage `json:"payload"`
	OriginalRequest json.RawMessage `json:"original_request,omitempty"`
	StorageJSON     json.RawMessage `json:"storage_json"`
	Headers         http.Header     `json:"headers,omitempty"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
}

// ExecutorResponse 执行器响应
type ExecutorResponse struct {
	Headers http.Header     `json:"headers,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// ExecutorStreamRequest 流式请求
type ExecutorStreamRequest struct {
	ExecutorRequest
	StreamID       string `json:"stream_id,omitempty"`
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

// ExecutorStreamChunk 流式响应单块
type ExecutorStreamChunk struct {
	Payload json.RawMessage `json:"payload"`
}

// StreamResponse 同步流式响应包装
type StreamResponse struct {
	Headers http.Header           `json:"headers,omitempty"`
	Chunks  []ExecutorStreamChunk `json:"chunks,omitempty"`
}

// ManagementRegistrationResponse 管理接口注册响应
type ManagementRegistrationResponse struct {
	Routes    []ManagementRoute `json:"routes,omitempty"`
	Resources []ResourceRoute   `json:"resources,omitempty"`
}

// ManagementRoute 管理 API 路由
type ManagementRoute struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Menu        string `json:"menu,omitempty"`
	Description string `json:"description,omitempty"`
}

// ResourceRoute 嵌入式资源路由 (Web UI)
type ResourceRoute struct {
	Path        string `json:"path"`
	Menu        string `json:"menu,omitempty"`
	Description string `json:"description,omitempty"`
}

// ManagementRequest 传入的管理请求
type ManagementRequest struct {
	Method  string      `json:"method"`
	Path    string      `json:"path"`
	Headers http.Header `json:"headers"`
	Query   url.Values  `json:"query"`
	Body    []byte      `json:"body"`
}

// ManagementResponse 管理响应
type ManagementResponse struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
}

// QuotaDescribeResponse 配额支持描述
type QuotaDescribeResponse struct {
	SupportedProviders []string `json:"supported_providers"`
	DisplayName        string   `json:"display_name"`
	SupportsReset      bool     `json:"supports_reset"`
}

// QuotaFetchRequest 配额查询请求
type QuotaFetchRequest struct {
	AuthID      string          `json:"auth_id"`
	Provider    string          `json:"provider"`
	StorageJSON json.RawMessage `json:"storage_json"`
}

// QuotaFetchResponse 配额查询响应
type QuotaFetchResponse struct {
	Provider       string         `json:"provider"`
	RemainingQuota float64        `json:"remaining_quota"`
	TotalQuota     float64        `json:"total_quota"`
	IsUnlimited    bool           `json:"is_unlimited"`
	ResetAt        time.Time      `json:"reset_at,omitempty"`
	Currency       string         `json:"currency,omitempty"`
	QuotaType      string         `json:"quota_type,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
}
