package cpa

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"workbuddy-ai-cliproxy/internal/auth"
)

func TestSchemaVersionNegotiation(t *testing.T) {
	dispatcher := GetDispatcher()

	cases := []struct {
		inputHostVersion uint32
		expectedVersion  uint32
	}{
		{inputHostVersion: 0, expectedVersion: 1},
		{inputHostVersion: 1, expectedVersion: 1},
		{inputHostVersion: 3, expectedVersion: 3},
		{inputHostVersion: 6, expectedVersion: 6},
		{inputHostVersion: 7, expectedVersion: 6}, // Capped at MaxSupportedSchemaVersion = 6
		{inputHostVersion: 8, expectedVersion: 6},
	}

	for _, tc := range cases {
		reqBytes, _ := json.Marshal(LifecycleRequest{SchemaVersion: tc.inputHostVersion})
		respBytes, err := dispatcher.HandleMethod(context.Background(), MethodPluginRegister, reqBytes, nil)
		if err != nil {
			t.Fatalf("HandleMethod plugin.register failed: %v", err)
		}

		var env Envelope
		if err := json.Unmarshal(respBytes, &env); err != nil {
			t.Fatalf("unmarshal envelope failed: %v", err)
		}
		if !env.OK {
			t.Fatalf("envelope not ok: %v", env.Error)
		}

		var reg Registration
		if err := json.Unmarshal(env.Result, &reg); err != nil {
			t.Fatalf("unmarshal registration failed: %v", err)
		}

		if reg.SchemaVersion != tc.expectedVersion {
			t.Errorf("hostVersion %d: expected schema_version %d, got %d",
				tc.inputHostVersion, tc.expectedVersion, reg.SchemaVersion)
		}

		if !reg.Capabilities.ModelProvider || !reg.Capabilities.AuthProvider || !reg.Capabilities.Executor {
			t.Errorf("expected core capabilities to be true")
		}
	}
}

func TestModelStaticAndPrefixes(t *testing.T) {
	store := auth.GetStore()
	store.UpsertInMemory(&auth.WorkBuddyAuth{ID: "test-cn", Realm: auth.RealmCN, Nickname: "国内号"})
	store.UpsertInMemory(&auth.WorkBuddyAuth{ID: "test-global", Realm: auth.RealmGlobal, Nickname: "国际号"})

	dispatcher := GetDispatcher()
	respBytes, err := dispatcher.HandleMethod(context.Background(), MethodModelStatic, nil, nil)
	if err != nil {
		t.Fatalf("handle models failed: %v", err)
	}

	var env Envelope
	_ = json.Unmarshal(respBytes, &env)
	if !env.OK {
		t.Fatalf("envelope not ok: %v", env.Error)
	}

	var resp ModelResponse
	_ = json.Unmarshal(env.Result, &resp)

	if resp.Provider != "workbuddyai" {
		t.Errorf("expected provider 'workbuddyai', got %s", resp.Provider)
	}

	modelMap := make(map[string]ModelInfo)
	for _, m := range resp.Models {
		modelMap[m.ID] = m
	}

	// 验证国内区核心模型及前缀
	if _, ok := modelMap["hy3"]; !ok {
		t.Errorf("expected model hy3 to be present")
	}
	if _, ok := modelMap["cn/hy3"]; !ok {
		t.Errorf("expected prefixed model cn/hy3 to be present")
	}
	if _, ok := modelMap["glm-5.2"]; !ok {
		t.Errorf("expected model glm-5.2 to be present")
	}

	// 验证国际区核心模型及前缀
	if _, ok := modelMap["gpt-6-astra"]; !ok {
		t.Errorf("expected model gpt-6-astra to be present")
	}
	if _, ok := modelMap["cb/gpt-6-astra"]; !ok {
		t.Errorf("expected prefixed model cb/gpt-6-astra to be present")
	}
	if _, ok := modelMap["en/gpt-6-astra"]; !ok {
		t.Errorf("expected prefixed model en/gpt-6-astra to be present")
	}
}

func TestAuthParseSingleAndMulti(t *testing.T) {
	dispatcher := GetDispatcher()

	// 1. 测试单账号 CN 解析
	cnPayload := `{
		"auth": {
			"accessToken": "eyCNTestToken123",
			"refreshToken": "rfCNTestToken123",
			"expiresAt": 1893456000,
			"domain": "copilot.tencent.com"
		},
		"account": {
			"uid": "10001",
			"nickname": "腾讯测试号"
		}
	}`

	req1 := AuthParseRequest{
		Provider: "workbuddy",
		FileName: "workbuddy.json",
		RawJSON:  json.RawMessage([]byte(cnPayload)),
	}
	req1Bytes, _ := json.Marshal(req1)
	resp1Bytes, err := dispatcher.HandleMethod(context.Background(), MethodAuthParse, req1Bytes, nil)
	if err != nil {
		t.Fatalf("parse cn auth failed: %v", err)
	}

	var env1 Envelope
	_ = json.Unmarshal(resp1Bytes, &env1)
	var parseResp1 AuthParseResponse
	_ = json.Unmarshal(env1.Result, &parseResp1)

	if !parseResp1.Handled {
		t.Fatalf("expected handled true for CN auth")
	}
	if parseResp1.Auth.Prefix != "cn/" {
		t.Errorf("expected prefix 'cn/', got %s", parseResp1.Auth.Prefix)
	}

	// 2. 测试单文件多账号数组展开
	multiPayload := `{
		"accounts": [
			{
				"type": "workbuddy",
				"access_token": "token-acc-1",
				"realm": "cn",
				"user_id": "cn-user-1",
				"nickname": "国内账号1"
			},
			{
				"type": "workbuddy",
				"access_token": "token-acc-2",
				"realm": "global",
				"user_id": "intl-user-2",
				"nickname": "国际账号2"
			}
		]
	}`

	req2 := AuthParseRequest{
		Provider: "workbuddy",
		FileName: "workbuddy-multi.json",
		RawJSON:  json.RawMessage([]byte(multiPayload)),
	}
	req2Bytes, _ := json.Marshal(req2)
	resp2Bytes, err := dispatcher.HandleMethod(context.Background(), MethodAuthParse, req2Bytes, nil)
	if err != nil {
		t.Fatalf("parse multi auth failed: %v", err)
	}

	var env2 Envelope
	_ = json.Unmarshal(resp2Bytes, &env2)
	var parseResp2 AuthParseResponse
	_ = json.Unmarshal(env2.Result, &parseResp2)

	if !parseResp2.Handled {
		t.Fatalf("expected handled true for multi auth")
	}
	if len(parseResp2.Auths) != 2 {
		t.Fatalf("expected 2 auths expanded, got %d", len(parseResp2.Auths))
	}
	if parseResp2.Auths[0].Prefix != "cn/" || parseResp2.Auths[1].Prefix != "cb/" {
		t.Errorf("expected prefixes 'cn/' and 'cb/', got %s, %s",
			parseResp2.Auths[0].Prefix, parseResp2.Auths[1].Prefix)
	}

	// 3. 验证内部 Store 已同步注册多账号
	store := auth.GetStore()
	if store.GetByID(parseResp2.Auths[0].ID) == nil {
		t.Errorf("expected acc1 to be registered in store")
	}
	if store.GetByID(parseResp2.Auths[1].ID) == nil {
		t.Errorf("expected acc2 to be registered in store")
	}
}

func TestManagementRegisterAndHandle(t *testing.T) {
	dispatcher := GetDispatcher()

	// 1. 测试路由注册
	regRespBytes, err := dispatcher.HandleMethod(context.Background(), MethodManagementRegister, nil, nil)
	if err != nil {
		t.Fatalf("management register failed: %v", err)
	}

	var env Envelope
	_ = json.Unmarshal(regRespBytes, &env)
	var regResp ManagementRegistrationResponse
	_ = json.Unmarshal(env.Result, &regResp)

	if len(regResp.Routes) == 0 {
		t.Errorf("expected management routes to be registered")
	}
	if len(regResp.Resources) != 1 || regResp.Resources[0].Path != "/ui" {
		t.Errorf("expected only single resource route /ui to be registered, got %d", len(regResp.Resources))
	}

	// 2. 测试 management.handle 分发到 Gin 内部获取 /models
	handleReq := ManagementRequest{
		Method: "GET",
		Path:   "/models",
	}
	handleReqBytes, _ := json.Marshal(handleReq)
	handleRespBytes, err := dispatcher.HandleMethod(context.Background(), MethodManagementHandle, handleReqBytes, nil)
	if err != nil {
		t.Fatalf("management handle failed: %v", err)
	}

	var envHandle Envelope
	_ = json.Unmarshal(handleRespBytes, &envHandle)
	var mgmtResp ManagementResponse
	_ = json.Unmarshal(envHandle.Result, &mgmtResp)

	if mgmtResp.StatusCode != 200 {
		t.Errorf("expected status 200 from /models, got %d", mgmtResp.StatusCode)
	}

	// 3. 核心断言：测试 /v0/resource/plugins/workbuddy/ui 请求直接返回 200 状态与自包含 HTML，杜绝 301 重定向
	uiPaths := []string{
		"/v0/resource/plugins/workbuddy/ui",
		"/v0/resource/plugins/workbuddyai/ui",
		"/ui",
	}

	for _, p := range uiPaths {
		uiReq := ManagementRequest{
			Method: "GET",
			Path:   p,
		}
		uiReqBytes, _ := json.Marshal(uiReq)
		uiRespBytes, err := dispatcher.HandleMethod(context.Background(), MethodManagementHandle, uiReqBytes, nil)
		if err != nil {
			t.Fatalf("management handle ui (%s) failed: %v", p, err)
		}

		var envUI Envelope
		_ = json.Unmarshal(uiRespBytes, &envUI)
		var uiResp ManagementResponse
		_ = json.Unmarshal(envUI.Result, &uiResp)

		if uiResp.StatusCode != 200 {
			t.Errorf("path %s: expected status 200, got %d", p, uiResp.StatusCode)
		}
		contentType := uiResp.Headers.Get("Content-Type")
		if contentType != "text/html; charset=utf-8" {
			t.Errorf("path %s: expected html content type, got %s", p, contentType)
		}
		bodyStr := string(uiResp.Body)
		if !strings.Contains(bodyStr, "WorkBuddy AI 控制台") {
			t.Errorf("path %s: expected HTML body to contain title", p)
		}
		if !strings.Contains(bodyStr, "<style>") || !strings.Contains(bodyStr, "<script>") {
			t.Errorf("path %s: expected embedded <style> and <script> tags", p)
		}
		if strings.Contains(bodyStr, "Moved Permanently") {
			t.Errorf("path %s: MUST NOT contain Moved Permanently redirect link", p)
		}
	}
}

func TestQuotaDescribeAndFetch(t *testing.T) {
	dispatcher := GetDispatcher()

	// Describe
	descBytes, err := dispatcher.HandleMethod(context.Background(), MethodQuotaDescribe, nil, nil)
	if err != nil {
		t.Fatalf("quota describe failed: %v", err)
	}

	var env Envelope
	_ = json.Unmarshal(descBytes, &env)
	var descResp QuotaDescribeResponse
	_ = json.Unmarshal(env.Result, &descResp)

	if len(descResp.SupportedProviders) == 0 {
		t.Errorf("expected supported providers in quota describe")
	}

	// Fetch with valid storage
	store := auth.GetStore()
	testAcc := &auth.WorkBuddyAuth{
		Type:        "workbuddy",
		ID:          "quota-test-acc",
		Realm:       auth.RealmCN,
		AccessToken: "test-token",
		Credits:     888.5,
	}
	store.UpsertInMemory(testAcc)

	fetchReq := QuotaFetchRequest{
		AuthID:   "quota-test-acc",
		Provider: "workbuddy",
	}
	fetchReqBytes, _ := json.Marshal(fetchReq)
	fetchRespBytes, err := dispatcher.HandleMethod(context.Background(), MethodQuotaFetch, fetchReqBytes, nil)
	if err != nil {
		t.Fatalf("quota fetch failed: %v", err)
	}

	var envFetch Envelope
	_ = json.Unmarshal(fetchRespBytes, &envFetch)
	var fetchResp QuotaFetchResponse
	_ = json.Unmarshal(envFetch.Result, &fetchResp)

	if fetchResp.RemainingQuota != 888.5 {
		t.Errorf("expected quota 888.5, got %f", fetchResp.RemainingQuota)
	}
	if len(fetchResp.Summary) == 0 || fetchResp.Summary[0].Value != 888.5 {
		t.Errorf("expected summary metric with value 888.5, got %v", fetchResp.Summary)
	}

	// 验证带 provider 前缀的 AuthID 检索 (匹配 CPAMP 传参)
	fetchReqPrefix := QuotaFetchRequest{
		AuthID:   "workbuddy/quota-test-acc",
		Provider: "workbuddy",
	}
	fetchReqPrefixBytes, _ := json.Marshal(fetchReqPrefix)
	fetchRespPrefixBytes, errPrefix := dispatcher.HandleMethod(context.Background(), MethodQuotaFetch, fetchReqPrefixBytes, nil)
	if errPrefix != nil {
		t.Fatalf("quota fetch with prefix failed: %v", errPrefix)
	}

	var envPrefix Envelope
	_ = json.Unmarshal(fetchRespPrefixBytes, &envPrefix)
	var fetchRespPrefix QuotaFetchResponse
	_ = json.Unmarshal(envPrefix.Result, &fetchRespPrefix)

	if fetchRespPrefix.RemainingQuota != 888.5 {
		t.Errorf("expected quota 888.5 with prefix, got %f", fetchRespPrefix.RemainingQuota)
	}
}
