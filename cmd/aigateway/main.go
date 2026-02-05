package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/api"
	"github.com/felipepmaragno/ai-gateway/internal/auth"
	"github.com/felipepmaragno/ai-gateway/internal/budget"
	"github.com/felipepmaragno/ai-gateway/internal/cache"
	"github.com/felipepmaragno/ai-gateway/internal/config"
	"github.com/felipepmaragno/ai-gateway/internal/cost"
	"github.com/felipepmaragno/ai-gateway/internal/metrics"
	"github.com/felipepmaragno/ai-gateway/internal/provider/anthropic"
	"github.com/felipepmaragno/ai-gateway/internal/provider/bedrock"
	"github.com/felipepmaragno/ai-gateway/internal/provider/ollama"
	"github.com/felipepmaragno/ai-gateway/internal/provider/openai"
	"github.com/felipepmaragno/ai-gateway/internal/ratelimit"
	"github.com/felipepmaragno/ai-gateway/internal/repository"
	"github.com/felipepmaragno/ai-gateway/internal/router"
	"github.com/felipepmaragno/ai-gateway/internal/telemetry"
	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	setupLogger(cfg.LogLevel)

	slog.Info("starting AI Gateway", "addr", cfg.Addr, "version", "0.5.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTelemetry, telemetryErr := telemetry.Init(ctx, "ai-gateway", cfg.OTLPEndpoint)
	if telemetryErr != nil {
		slog.Warn("failed to initialize telemetry", "error", telemetryErr)
	}
	defer func() {
		if shutdownTelemetry != nil {
			_ = shutdownTelemetry(ctx)
		}
	}()

	var tenantRepo repository.TenantRepository
	var costTracker cost.Tracker
	var db *sql.DB

	if cfg.DatabaseURL != "" {
		db, err = sql.Open("postgres", cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("connect to database: %w", err)
		}
		defer db.Close()

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		if pingErr := db.PingContext(ctx); pingErr != nil {
			return fmt.Errorf("ping database: %w", pingErr)
		}

		tenantRepo = repository.NewPostgresTenantRepository(db)
		costTracker = repository.NewPostgresUsageRepository(db)
		slog.Info("using postgresql storage")
	} else {
		tenantRepo = repository.NewInMemoryTenantRepository()
		costTracker = cost.NewInMemoryTracker()
		slog.Info("using in-memory storage")
	}

	var rateLimiter ratelimit.RateLimiter
	if cfg.RedisURL != "" {
		rateLimiter, err = ratelimit.NewRedisRateLimiter(cfg.RedisURL)
		if err != nil {
			return fmt.Errorf("connect to redis: %w", err)
		}
		slog.Info("using redis rate limiter", "url", cfg.RedisURL)
	} else {
		rateLimiter = ratelimit.NewInMemoryRateLimiter()
		slog.Info("using in-memory rate limiter")
	}

	providers := make(map[string]router.Provider)

	if cfg.OpenAIAPIKey != "" {
		providers["openai"] = openai.New(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL)
		slog.Info("registered provider", "provider", "openai")
	}

	if cfg.OllamaBaseURL != "" {
		providers["ollama"] = ollama.New(cfg.OllamaBaseURL)
		slog.Info("registered provider", "provider", "ollama", "url", cfg.OllamaBaseURL)
	}

	if cfg.AnthropicAPIKey != "" {
		providers["anthropic"] = anthropic.New(cfg.AnthropicAPIKey)
		slog.Info("registered provider", "provider", "anthropic")
	}

	if cfg.AWSRegion != "" {
		bedrockProvider, bedrockErr := bedrock.New(ctx, cfg.AWSRegion)
		if bedrockErr != nil {
			slog.Warn("failed to initialize bedrock provider", "error", bedrockErr)
		} else {
			providers["bedrock"] = bedrockProvider
			slog.Info("registered provider", "provider", "bedrock", "region", cfg.AWSRegion)
		}
	}

	if len(providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	// Initialize circuit breaker state metrics for all providers
	for providerName := range providers {
		metrics.SetCircuitBreakerState(providerName, 0) // 0 = closed (healthy)
	}

	providerRouter := router.New(providers, cfg.DefaultProvider)

	var responseCache cache.Cache
	if cfg.RedisURL != "" {
		responseCache, err = cache.NewRedisCache(cfg.RedisURL)
		if err != nil {
			slog.Warn("failed to connect to redis for cache, using in-memory", "error", err)
			responseCache = cache.NewInMemoryCache()
		} else {
			slog.Info("using redis cache")
		}
	} else {
		responseCache = cache.NewInMemoryCache()
		slog.Info("using in-memory cache")
	}

	budgetMonitor := budget.NewMonitor(costTracker, budget.DefaultThresholds())
	budgetMonitor.OnAlert(budget.LogAlertHandler)

	handler := api.NewHandler(api.HandlerConfig{
		TenantRepo:    tenantRepo,
		RateLimiter:   rateLimiter,
		Router:        providerRouter,
		Cache:         responseCache,
		CacheTTL:      5 * time.Minute,
		CostTracker:   costTracker,
		BudgetMonitor: budgetMonitor,
	})

	adminHandler := api.NewAdminHandler(tenantRepo)

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	if cfg.AdminAuthEnabled {
		var adminUserRepo auth.AdminUserRepository
		if db != nil {
			adminUserRepo = auth.NewPostgresAdminUserRepository(db)
		} else {
			adminUserRepo = auth.NewInMemoryAdminUserRepository()
		}
		authenticator := auth.NewAuthenticator(adminUserRepo)
		rbacMiddleware := auth.NewRBACMiddleware(authenticator)
		mux.Handle("/admin/", rbacMiddleware.RequireAuth(adminHandler))
		slog.Info("admin API authentication enabled")
	} else {
		mux.Handle("/admin/", adminHandler)
		slog.Info("admin API authentication disabled")
	}

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server stopped")
	return nil
}

func setupLogger(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))
}
