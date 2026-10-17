//go:build cgo

package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

// Wrappers so Go can invoke the host function-pointer table via cgo.
static int wb_call_host(cliproxy_host_api* api, const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	if (api == NULL || api->call == NULL) return -1;
	return api->call(api->host_ctx, method, request, request_len, response);
}

static void wb_free_host_buffer(cliproxy_host_api* api, void* ptr, size_t len) {
	if (api != NULL && api->free_buffer != NULL && ptr != NULL) {
		api->free_buffer(ptr, len);
	}
}

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"unsafe"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/internal/cpa"
	"workbuddy-ai-cliproxy/internal/growth"
	"workbuddy-ai-cliproxy/internal/management"
	"workbuddy-ai-cliproxy/internal/models"
	"workbuddy-ai-cliproxy/pkg/version"

	"github.com/gin-gonic/gin"
)

var (
	cgoHostAPI  *C.cliproxy_host_api
	cgoBridgeMu sync.RWMutex
)

// CGOHostBridge 封装宿主 C 函数指针桥接
type CGOHostBridge struct{}

func (b *CGOHostBridge) HostCall(method string, request []byte) ([]byte, error) {
	cgoBridgeMu.RLock()
	api := cgoHostAPI
	cgoBridgeMu.RUnlock()

	if api == nil || api.call == nil {
		return nil, fmt.Errorf("cgo host API unavailable")
	}

	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))

	var cReq unsafe.Pointer
	var reqLen C.size_t
	if len(request) > 0 {
		cReq = C.CBytes(request)
		defer C.free(cReq)
		reqLen = C.size_t(len(request))
	}

	var resp C.cliproxy_buffer
	rc := C.wb_call_host(api, cMethod, (*C.uint8_t)(cReq), reqLen, &resp)

	var out []byte
	if resp.ptr != nil && resp.len > 0 {
		out = C.GoBytes(resp.ptr, C.int(resp.len))
	}
	if resp.ptr != nil && api.free_buffer != nil {
		C.wb_free_host_buffer(api, resp.ptr, resp.len)
	}

	if rc != 0 {
		return out, fmt.Errorf("host call %s returned %d", method, int(rc))
	}
	return out, nil
}

func (b *CGOHostBridge) StreamEmit(streamID string, payload []byte) error {
	if streamID == "" {
		return fmt.Errorf("no stream id")
	}
	body, _ := json.Marshal(map[string]any{"stream_id": streamID, "payload": payload})
	_, err := b.HostCall(cpa.MethodHostStreamEmit, body)
	return err
}

func (b *CGOHostBridge) StreamClose(streamID string) {
	if streamID == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{"stream_id": streamID})
	_, _ = b.HostCall(cpa.MethodHostStreamClose, body)
}

var defaultCGOBridge = &CGOHostBridge{}

// =============================================================================
// 1. 标准 CLIProxyAPI v7 / v8 C ABI 导出
// =============================================================================

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}

	cgoBridgeMu.Lock()
	cgoHostAPI = host
	cgoBridgeMu.Unlock()

	log.Printf("[workbuddy] cliproxy_plugin_init: %s", version.Info())

	cfg := config.Get()
	_ = cfg.EnsureDataDir()
	_ = cfg.EnsureAuthDir()

	authDir := config.GetAuthDir()
	dataDir := config.GetDataDir()
	log.Printf("[workbuddy] auth dir: %s", authDir)
	log.Printf("[workbuddy] data dir: %s", dataDir)

	// 初始化本地凭据仓库 (绑定持久卷绝对路径)
	store := auth.GetStore()
	if err := store.Init(authDir); err != nil {
		log.Printf("[workbuddy] warning: init auth store failed: %v", err)
	} else {
		log.Printf("[workbuddy] loaded %d auth accounts successfully from %s", len(store.List()), authDir)
	}

	// 启动后台福利调度协程
	growth.StartScheduler()
	log.Printf("[workbuddy] growth scheduler running (daily_bonus: %v, checkin_auto: %v)",
		cfg.DailyBonus, cfg.CheckinAuto)

	// 初始化模型注册表与分发器
	_ = models.GetRegistry()
	_ = cpa.GetDispatcher()

	plugin.abi_version = C.uint32_t(cpa.ABIVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)

	log.Printf("[workbuddy] plugin successfully loaded (CPA v7/v8 ABI ready)")
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, []byte(`{"ok":false,"error":{"code":"invalid_method","message":"method is required"}}`))
		return 1
	}

	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}

	methodStr := C.GoString(method)
	dispatcher := cpa.GetDispatcher()
	raw, err := dispatcher.HandleMethod(context.Background(), methodStr, requestBytes, defaultCGOBridge)
	if err != nil {
		errJSON, _ := json.Marshal(map[string]any{
			"ok": false,
			"error": map[string]any{
				"code":    "plugin_error",
				"message": err.Error(),
			},
		})
		writeResponse(response, errJSON)
		return 1
	}

	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, len C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {
	log.Printf("[workbuddy] cliproxyPluginShutdown: stopping background schedulers...")
	growth.StopScheduler()
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}

// =============================================================================
// 2. 向下兼容 CPA v6 / 自定义宿主符号导出
// =============================================================================

//export InitPlugin
func InitPlugin(ctxPtr unsafe.Pointer) unsafe.Pointer {
	log.Printf("[workbuddy] legacy InitPlugin called: %s", version.Info())

	cfg := config.Get()
	_ = cfg.EnsureDataDir()
	_ = cfg.EnsureAuthDir()

	authDir := config.GetAuthDir()
	dataDir := config.GetDataDir()
	log.Printf("[workbuddy] auth dir: %s", authDir)
	log.Printf("[workbuddy] data dir: %s", dataDir)

	store := auth.GetStore()
	_ = store.Init(authDir)

	growth.StartScheduler()
	_ = models.GetRegistry()
	_ = cpa.GetDispatcher()

	log.Printf("[workbuddy] legacy InitPlugin ready")
	return ctxPtr
}

//export RegisterRoutes
func RegisterRoutes(routerGroupPtr unsafe.Pointer) {
	if routerGroupPtr == nil {
		return
	}
	group := (*gin.RouterGroup)(routerGroupPtr)
	management.MountRoutes(group)
	log.Printf("[workbuddy] legacy RegisterRoutes mounted on: %s", group.BasePath())
}

//export UnloadPlugin
func UnloadPlugin() {
	log.Printf("[workbuddy] legacy UnloadPlugin: stopping schedulers...")
	growth.StopScheduler()
}
