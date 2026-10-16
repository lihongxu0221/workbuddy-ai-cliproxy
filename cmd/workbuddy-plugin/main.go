package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"log"
	"unsafe"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/internal/executor"
	"workbuddy-ai-cliproxy/internal/growth"
	"workbuddy-ai-cliproxy/internal/management"
	"workbuddy-ai-cliproxy/internal/models"
	"workbuddy-ai-cliproxy/pkg/version"

	"github.com/gin-gonic/gin"
)

func main() {
	// C-shared 库必需 main 函数作为占位
}

// PluginContext CPA 宿主加载插件时传入的核心上下文引用 (伪代码映射)
type PluginContext struct {
	ConfigDir  string
	AuthDir    string
	Engine     *gin.Engine
	HostConfig map[string]any
}

//export InitPlugin
func InitPlugin(ctxPtr unsafe.Pointer) unsafe.Pointer {
	log.Printf("[workbuddy] initializing plugin: %s", version.Info())

	cfg := config.Get()
	_ = cfg.EnsureDataDir()

	// 初始化认证存储仓库
	store := auth.GetStore()
	if err := store.Init("auths"); err != nil {
		log.Printf("[workbuddy] warning: init auth store failed: %v", err)
	} else {
		accounts := store.List()
		log.Printf("[workbuddy] loaded %d auth accounts successfully", len(accounts))
	}

	// 启动后台定时打卡与签到守护协程
	growth.StartScheduler()
	log.Printf("[workbuddy] growth scheduler running (daily_bonus: %v, checkin_auto: %v)",
		cfg.DailyBonus, cfg.CheckinAuto)

	// 初始化模型注册表
	_ = models.GetRegistry()

	// 创建核心执行器
	_ = executor.NewWorkBuddyExecutor()

	log.Printf("[workbuddy] plugin successfully loaded and ready to serve")
	return ctxPtr
}

//export RegisterRoutes
func RegisterRoutes(routerGroupPtr unsafe.Pointer) {
	if routerGroupPtr == nil {
		return
	}
	group := (*gin.RouterGroup)(routerGroupPtr)
	management.MountRoutes(group)
	log.Printf("[workbuddy] management routes mounted on: %s", group.BasePath())
}

//export UnloadPlugin
func UnloadPlugin() {
	log.Printf("[workbuddy] unloading plugin, stopping background daemons...")
	growth.StopScheduler()
}
