package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
	"workbuddy-ai-cliproxy/internal/cpa"
	"workbuddy-ai-cliproxy/internal/growth"
	"workbuddy-ai-cliproxy/internal/management"
	"workbuddy-ai-cliproxy/internal/models"
	"workbuddy-ai-cliproxy/pkg/version"

	"github.com/gin-gonic/gin"
)

func main() {
	// 判断是否以独立命令行服务模式运行
	port := flag.Int("port", 8318, "Standalone HTTP proxy server port")
	dataDir := flag.String("data-dir", ".codebuddy_data", "Local runtime data directory")
	authDir := flag.String("auth-dir", "auths", "Auth credentials directory")
	standalone := flag.Bool("standalone", false, "Force standalone HTTP server mode")
	showVersion := flag.Bool("version", false, "Print plugin version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Info())
		return
	}

	// 当作为 C-shared 动态库加载时，宿主不会传入 standalone 标志，main 函数直接返回
	if !*standalone && os.Getenv("WORKBUDDY_STANDALONE") != "1" && len(os.Args) <= 1 {
		return
	}

	log.Printf("[workbuddy] starting standalone HTTP proxy server: %s", version.Info())

	cfg := config.Get()
	if *dataDir != "" && *dataDir != ".codebuddy_data" {
		cfg.DataDir = *dataDir
	}
	if *authDir != "" && *authDir != "auths" {
		cfg.AuthDir = *authDir
	}
	_ = cfg.EnsureDataDir()
	_ = cfg.EnsureAuthDir()

	authDirToUse := config.GetAuthDir()
	dataDirToUse := config.GetDataDir()
	log.Printf("[workbuddy] auth dir: %s", authDirToUse)
	log.Printf("[workbuddy] data dir: %s", dataDirToUse)

	store := auth.GetStore()
	if err := store.Init(authDirToUse); err != nil {
		log.Printf("[workbuddy] warning: init auth store failed: %v", err)
	} else {
		log.Printf("[workbuddy] loaded %d auth accounts successfully from %s", len(store.List()), authDirToUse)
	}

	// 启动后台福利调度协程
	growth.StartScheduler()
	log.Printf("[workbuddy] growth scheduler running (daily_bonus: %v, checkin_auto: %v)",
		cfg.DailyBonus, cfg.CheckinAuto)

	_ = models.GetRegistry()
	dispatcher := cpa.GetDispatcher()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 1. 挂载管理控制台与 API
	management.MountRoutes(r.Group(""))

	// 2. 提供 OpenAI 兼容的 /v1/models 与 /v1/chat/completions 接口 (供直接调用或 CPA v6 接入)
	v1 := r.Group("/v1")
	{
		v1.GET("/models", func(c *gin.Context) {
			rawResp, err := dispatcher.HandleMethod(c.Request.Context(), cpa.MethodModelStatic, nil, nil)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.Data(http.StatusOK, "application/json", rawResp)
		})

		v1.POST("/chat/completions", func(c *gin.Context) {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
				return
			}

			var reqObj map[string]any
			_ = json.Unmarshal(bodyBytes, &reqObj)

			model, _ := reqObj["model"].(string)
			isStream, _ := reqObj["stream"].(bool)

			execReq := cpa.ExecutorRequest{
				Model:   model,
				Payload: bodyBytes,
				Headers: c.Request.Header,
			}
			execReqBytes, _ := json.Marshal(execReq)

			if isStream {
				c.Header("Content-Type", "text/event-stream")
				c.Header("Cache-Control", "no-cache")
				c.Header("Connection", "keep-alive")

				flusher, ok := c.Writer.(http.Flusher)
				if !ok {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
					return
				}

				bridge := &streamProxyBridge{
					writer:  c.Writer,
					flusher: flusher,
					doneCh:  make(chan struct{}),
				}

				_, _ = dispatcher.HandleMethod(c.Request.Context(), cpa.MethodExecutorExecuteStream, execReqBytes, bridge)
				<-bridge.doneCh
				return
			}

			rawResp, err := dispatcher.HandleMethod(c.Request.Context(), cpa.MethodExecutorExecute, execReqBytes, nil)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}

			var env cpa.Envelope
			_ = json.Unmarshal(rawResp, &env)
			if !env.OK {
				errMsg := "execution failed"
				if env.Error != nil {
					errMsg = env.Error.Message
				}
				c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
				return
			}

			var execResp cpa.ExecutorResponse
			_ = json.Unmarshal(env.Result, &execResp)
			c.Data(http.StatusOK, "application/json", execResp.Payload)
		})
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("[workbuddy] standalone server listening at http://127.0.0.1%s", addr)
		log.Printf("[workbuddy] management web console available at http://127.0.0.1%s/ui", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[workbuddy] server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("[workbuddy] shutting down standalone server...")
	growth.StopScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

type streamProxyBridge struct {
	writer  io.Writer
	flusher http.Flusher
	doneCh  chan struct{}
	once    sync.Once
}

func (s *streamProxyBridge) HostCall(method string, request []byte) ([]byte, error) {
	return nil, nil
}

func (s *streamProxyBridge) StreamEmit(streamID string, payload []byte) error {
	_, err := fmt.Fprintf(s.writer, "data: %s\n\n", string(payload))
	s.flusher.Flush()
	return err
}

func (s *streamProxyBridge) StreamClose(streamID string) {
	s.once.Do(func() {
		_, _ = fmt.Fprint(s.writer, "data: [DONE]\n\n")
		s.flusher.Flush()
		close(s.doneCh)
	})
}
