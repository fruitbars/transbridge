// main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"transbridge/api/deeplx/translate_handler"
	"transbridge/api/openai"
	"transbridge/cache"
	"transbridge/config"
	"transbridge/internal/middleware"
	"transbridge/logger"
	"transbridge/service"
	"transbridge/translator"
)

func main() {
	// 命令行参数
	configFile := flag.String("config", "config.yml", "配置文件路径")
	flag.Parse()

	// 设置日志格式
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 加载配置
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化组件
	var cacheImpl cache.Cache
	if cfg.Cache.Enabled {
		if cacheImpl, err = initCache(cfg); err != nil {
			log.Fatalf("Failed to initialize cache: %v", err)
		}
	}

	// 初始化翻译日志
	var translLogger *logger.TranslationLogger
	if cfg.Log.Enabled {
		loggerOpts := logger.LoggerOptions{
			Enabled:     cfg.Log.Enabled,
			LogFilePath: cfg.Log.FilePath,
			MaxSize:     cfg.Log.MaxSize,    // 单位：MB
			MaxAge:      cfg.Log.MaxAge,     // 单位：天
			MaxBackups:  cfg.Log.MaxBackups, // 最大备份数量
			QueueSize:   cfg.Log.QueueSize,
		}

		var err error
		translLogger, err = logger.NewTranslationLogger(loggerOpts)
		if err != nil {
			log.Printf("Warning: Failed to initialize translation logger: %v", err)
		} else {
			log.Printf("Translation logger initialized: %s", cfg.Log.FilePath)
		}
	}

	// 初始化模型管理器
	modelManager, err := translator.NewModelManager(cfg.Providers)
	if err != nil {
		log.Fatalf("Failed to initialize model manager: %v", err)
	}

	// 初始化翻译服务
	translationService := service.NewTranslationService(modelManager, cacheImpl, translLogger)

	// 初始化 HTTP 服务器
	server := setupServer(cfg, translationService, modelManager)

	// 启动服务器
	go func() {
		log.Printf("Starting server on port %d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// 等待终止信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 优雅关闭
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	if translLogger != nil {
		if err := translLogger.Close(); err != nil {
			log.Printf("Error closing translation logger: %v", err)
		}
	}

	// 关闭缓存
	if cacheImpl != nil {
		if err := cacheImpl.Close(ctx); err != nil {
			log.Printf("Error closing cache: %v", err)
		}
	}

	log.Println("Server exited")
}

func setupServer(cfg *config.Config, translationService *service.TranslationService, modelManager *translator.ModelManager) *http.Server {
	// 创建路由
	mux := http.NewServeMux()

	// 创建处理器
	translationHandler := translate_handler.NewHandler(translationService, translate_handler.HandlerConfig{
		AuthTokens:     cfg.TransAPI.Tokens,
		PromptTemplate: cfg.Prompt.Template,
	})

	// 注册翻译接口
	translateEndpoint := middleware.Chain(
		translationHandler.HandleTranslation,
		middleware.Recovery,
		middleware.Logger,
		middleware.CORS,
	)
	mux.HandleFunc("/translate", translateEndpoint)

	mux.HandleFunc("/deepl/v2/translate",
		middleware.Chain(
			translationHandler.HandleDeepLTranslation,
			middleware.Recovery,
			middleware.Logger,
			middleware.CORS,
		),
	)

	mux.HandleFunc("/immersivel",
		middleware.Chain(
			translationHandler.HandleImmersiveLTranslation,
			middleware.Recovery,
			middleware.Logger,
			middleware.CORS,
		),
	)

	// 如果启用了 OpenAI 兼容接口，注册相关路由
	if cfg.OpenAI.CompatibleAPI.Enabled {
		openaiHandler := openai.NewOpenAIHandler(modelManager, cfg.OpenAI.CompatibleAPI.AuthTokens)

		basePath := cfg.OpenAI.CompatibleAPI.Path
		if basePath == "" {
			basePath = "/v1"
		}

		mux.HandleFunc(basePath+"/chat/completions",
			middleware.Chain(
				openaiHandler.HandleChatCompletion,
				middleware.Recovery,
				middleware.Logger,
				middleware.CORS,
			),
		)

		mux.HandleFunc(basePath+"/models",
			middleware.Chain(
				openaiHandler.HandleListModels,
				middleware.Recovery,
				middleware.Logger,
				middleware.CORS,
			),
		)
	}

	// 健康检查
	mux.HandleFunc("/health",
		middleware.Chain(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			middleware.Logger,
		),
	)

	// 创建服务器
	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// main.go 中的缓存初始化函数
func initCache(cfg *config.Config) (cache.Cache, error) {
	var caches []cache.Cache

	for _, cacheType := range cfg.Cache.Types {
		switch cacheType {
		case "memory":
			// 解析内存缓存TTL
			ttl := time.Hour // 默认1小时
			isPermanent := false

			if duration, ok := cfg.Cache.Memory.TTL.Duration(); ok {
				if duration < 0 {
					isPermanent = true
				} else {
					ttl = duration
				}
			}

			maxSize := cfg.Cache.Memory.MaxSize
			if maxSize <= 0 {
				maxSize = 10000 // 默认10000条
			}

			memoryCacheOptions := cache.MemoryCacheOptions{
				MaxSize:    maxSize,
				DefaultTTL: ttl,
				Permanent:  isPermanent,
			}

			caches = append(caches, cache.NewMemoryCache(memoryCacheOptions))

		case "redis":
			// 解析Redis缓存TTL
			ttl := 24 * time.Hour // 默认1天
			isPermanent := false

			if duration, ok := cfg.Cache.Redis.TTL.Duration(); ok {
				if duration < 0 {
					isPermanent = true
				} else {
					ttl = duration
				}
			}

			redisCacheOptions := cache.RedisCacheOptions{
				Host:       cfg.Cache.Redis.Host,
				Port:       cfg.Cache.Redis.Port,
				Password:   cfg.Cache.Redis.Password,
				DB:         cfg.Cache.Redis.DB,
				DefaultTTL: ttl,
				Permanent:  isPermanent,
			}

			caches = append(caches, cache.NewRedisCache(redisCacheOptions))

		case "bbolt", "bolt":
			ttl := 24 * time.Hour // 默认1天
			isPermanent := false

			if duration, ok := cfg.Cache.Bbolt.TTL.Duration(); ok {
				if duration < 0 {
					isPermanent = true
				} else {
					ttl = duration
				}
			}

			bboltCache, err := cache.NewBboltCache(cache.BboltCacheOptions{
				Path:       cfg.Cache.Bbolt.Path,
				DefaultTTL: ttl,
				Permanent:  isPermanent,
			})
			if err != nil {
				return nil, err
			}
			caches = append(caches, bboltCache)

		default:
			return nil, fmt.Errorf("unsupported cache type: %s", cacheType)
		}
	}

	if len(caches) == 0 {
		return nil, fmt.Errorf("no cache configured")
	}

	return cache.NewMultiCache(caches), nil
}
