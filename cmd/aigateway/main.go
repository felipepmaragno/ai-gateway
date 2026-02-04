package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/api"
	"github.com/felipepmaragno/ai-gateway/internal/cache"
	"github.com/felipepmaragno/ai-gateway/internal/config"
	"github.com/felipepmaragno/ai-gateway/internal/provider/anthropic"
	"github.com/felipepmaragno/ai-gateway/internal/provider/ollama"
	"github.com/felipepmaragno/ai-gateway/internal/provider/openai"
	"github.com/felipepmaragno/ai-gateway/internal/ratelimit"
	"github.com/felipepmaragno/ai-gateway/internal/repository"
	"github.com/felipepmaragno/ai-gateway/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)

	slog.Info("starting AI Gateway", "addr", cfg.Addr, "version", "0.2.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tenantRepo := repository.NewInMemoryTenantRepository()

	var rateLimiter ratelimit.RateLimiter
	if cfg.RedisURL != "" {
		rateLimiter, err = ratelimit.NewRedisRateLimiter(cfg.RedisURL)
		if err != nil {
			slog.Error("failed to connect to redis", "error", err)
			os.Exit(1)
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

	if len(providers) == 0 {
		slog.Error("no providers configured")
		os.Exit(1)
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

	handler := api.NewHandler(api.HandlerConfig{
		TenantRepo:  tenantRepo,
		RateLimiter: rateLimiter,
		Router:      providerRouter,
		Cache:       responseCache,
		CacheTTL:    5 * time.Minute,
	})

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
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
