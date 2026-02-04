package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/budget"
	"github.com/felipepmaragno/ai-gateway/internal/cache"
	"github.com/felipepmaragno/ai-gateway/internal/cost"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/felipepmaragno/ai-gateway/internal/metrics"
	"github.com/felipepmaragno/ai-gateway/internal/ratelimit"
	"github.com/felipepmaragno/ai-gateway/internal/repository"
	"github.com/felipepmaragno/ai-gateway/internal/router"
	"github.com/felipepmaragno/ai-gateway/internal/telemetry"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HandlerConfig struct {
	TenantRepo     repository.TenantRepository
	RateLimiter    ratelimit.RateLimiter
	Router         *router.Router
	Cache          cache.Cache
	CacheTTL       time.Duration
	CostCalculator *cost.Calculator
	CostTracker    cost.Tracker
	BudgetMonitor  *budget.Monitor
}

type Handler struct {
	tenantRepo     repository.TenantRepository
	rateLimiter    ratelimit.RateLimiter
	router         *router.Router
	cache          cache.Cache
	cacheTTL       time.Duration
	costCalculator *cost.Calculator
	costTracker    cost.Tracker
	budgetMonitor  *budget.Monitor
	mux            *http.ServeMux
}

func NewHandler(cfg HandlerConfig) *Handler {
	cacheTTL := cfg.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}

	costCalc := cfg.CostCalculator
	if costCalc == nil {
		costCalc = cost.NewCalculator()
	}

	h := &Handler{
		tenantRepo:     cfg.TenantRepo,
		rateLimiter:    cfg.RateLimiter,
		router:         cfg.Router,
		cache:          cfg.Cache,
		cacheTTL:       cacheTTL,
		costCalculator: costCalc,
		costTracker:    cfg.CostTracker,
		budgetMonitor:  cfg.BudgetMonitor,
		mux:            http.NewServeMux(),
	}

	h.mux.HandleFunc("POST /v1/chat/completions", h.handleChatCompletions)
	h.mux.HandleFunc("GET /v1/models", h.handleListModels)
	h.mux.HandleFunc("GET /v1/usage", h.handleUsage)
	h.mux.HandleFunc("GET /health", h.handleHealth)
	h.mux.HandleFunc("GET /health/live", h.handleHealthLive)
	h.mux.HandleFunc("GET /health/ready", h.handleHealthReady)
	h.mux.Handle("GET /metrics", promhttp.Handler())

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	ctx, span := telemetry.StartSpan(ctx, "chat.completions")
	defer span.End()

	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	traceID := telemetry.GetTraceID(ctx)

	apiKey := extractAPIKey(r)
	if apiKey == "" {
		metrics.RequestsTotal.WithLabelValues("", "", "", "unauthorized").Inc()
		writeError(w, http.StatusUnauthorized, "missing API key")
		return
	}

	tenant, err := h.tenantRepo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		slog.Warn("invalid API key", "error", err, "request_id", requestID)
		metrics.RequestsTotal.WithLabelValues("", "", "", "unauthorized").Inc()
		writeError(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	if h.budgetMonitor != nil {
		exceeded, err := h.budgetMonitor.IsBudgetExceeded(ctx, tenant)
		if err != nil {
			slog.Error("budget check error", "error", err, "request_id", requestID)
		} else if exceeded {
			slog.Warn("budget exceeded", "tenant_id", tenant.ID, "request_id", requestID)
			metrics.RequestsTotal.WithLabelValues(tenant.ID, "", "", "budget_exceeded").Inc()
			writeError(w, http.StatusPaymentRequired, "budget exceeded")
			return
		}
	}

	allowed, remaining, resetAt, err := h.rateLimiter.Allow(ctx, tenant.ID, tenant.RateLimitRPM)
	if err != nil {
		slog.Error("rate limiter error", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(tenant.RateLimitRPM))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("X-RateLimit-Reset", resetAt.Format(time.RFC3339))

	if !allowed {
		slog.Warn("rate limit exceeded", "tenant_id", tenant.ID, "request_id", requestID)
		metrics.RecordRateLimitHit(tenant.ID)
		metrics.RequestsTotal.WithLabelValues(tenant.ID, "", "", "rate_limited").Inc()
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

	var req domain.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.RequestsTotal.WithLabelValues(tenant.ID, "", "", "bad_request").Inc()
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	providerHint := r.Header.Get("X-Provider")
	skipCache := r.Header.Get("X-Skip-Cache") == "true"

	if req.Stream {
		provider, err := h.router.SelectProvider(ctx, providerHint, req.Model)
		if err != nil {
			slog.Error("provider selection failed", "error", err, "request_id", requestID)
			metrics.RequestsTotal.WithLabelValues(tenant.ID, "", req.Model, "no_provider").Inc()
			writeError(w, http.StatusBadGateway, "no provider available")
			return
		}
		h.handleStreamingResponse(w, r, provider, req, tenant, requestID, traceID, start)
		return
	}

	var cacheKey string
	if h.cache != nil && !skipCache {
		cacheKey = cache.GenerateCacheKey(req)
		if cached, ok := h.cache.Get(ctx, cacheKey); ok {
			latency := time.Since(start).Milliseconds()
			cached.Gateway = &domain.Gateway{
				Provider:  "cache",
				LatencyMs: latency,
				CostUSD:   0,
				CacheHit:  true,
				RequestID: requestID,
				TraceID:   traceID,
			}
			metrics.RecordCacheHit(tenant.ID)
			metrics.RecordRequest(tenant.ID, "cache", req.Model, "success", float64(latency)/1000)
			telemetry.AddCacheAttribute(span, true)
			slog.Info("cache hit",
				"request_id", requestID,
				"tenant_id", tenant.ID,
				"model", req.Model,
				"latency_ms", latency,
			)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Request-ID", requestID)
			w.Header().Set("X-Cache", "HIT")
			json.NewEncoder(w).Encode(cached)
			return
		}
		metrics.RecordCacheMiss(tenant.ID)
	}

	telemetry.AddCacheAttribute(span, false)

	providers, err := h.router.SelectProviderWithFallback(ctx, providerHint, req.Model)
	if err != nil {
		slog.Error("provider selection failed", "error", err, "request_id", requestID)
		metrics.RequestsTotal.WithLabelValues(tenant.ID, "", req.Model, "no_provider").Inc()
		writeError(w, http.StatusBadGateway, "no provider available")
		return
	}

	var resp *domain.ChatResponse
	var lastErr error
	var usedProvider router.Provider

	for _, provider := range providers {
		resp, lastErr = provider.ChatCompletion(ctx, req)
		if lastErr == nil {
			h.router.RecordSuccess(provider.ID())
			usedProvider = provider
			break
		}
		slog.Warn("provider failed, trying fallback",
			"provider", provider.ID(),
			"error", lastErr,
			"request_id", requestID,
		)
		h.router.RecordFailure(provider.ID())
		metrics.RecordProviderError(provider.ID(), "request_failed")
	}

	if resp == nil {
		slog.Error("all providers failed", "error", lastErr, "request_id", requestID)
		metrics.RequestsTotal.WithLabelValues(tenant.ID, "", req.Model, "provider_error").Inc()
		telemetry.AddErrorAttribute(span, lastErr)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("all providers failed: %v", lastErr))
		return
	}

	if h.cache != nil && cacheKey != "" {
		if err := h.cache.Set(ctx, cacheKey, resp, h.cacheTTL); err != nil {
			slog.Warn("failed to cache response", "error", err, "request_id", requestID)
		}
	}

	costUSD := h.costCalculator.Calculate(req.Model, resp.Usage)

	if h.costTracker != nil {
		record := cost.UsageRecord{
			TenantID:     tenant.ID,
			RequestID:    requestID,
			Model:        req.Model,
			Provider:     usedProvider.ID(),
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			CostUSD:      costUSD,
			Timestamp:    time.Now(),
		}
		if err := h.costTracker.Record(ctx, record); err != nil {
			slog.Warn("failed to record usage", "error", err, "request_id", requestID)
		}

		if h.budgetMonitor != nil {
			h.budgetMonitor.Check(ctx, tenant)
		}
	}

	latency := time.Since(start).Milliseconds()
	resp.Gateway = &domain.Gateway{
		Provider:  usedProvider.ID(),
		LatencyMs: latency,
		CostUSD:   costUSD,
		CacheHit:  false,
		RequestID: requestID,
		TraceID:   traceID,
	}

	metrics.RecordRequest(tenant.ID, usedProvider.ID(), req.Model, "success", float64(latency)/1000)
	metrics.RecordTokens(tenant.ID, usedProvider.ID(), req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	metrics.RecordCost(tenant.ID, usedProvider.ID(), req.Model, costUSD)

	telemetry.AddRequestAttributes(span, tenant.ID, usedProvider.ID(), req.Model, requestID)
	telemetry.AddTokenAttributes(span, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	telemetry.AddCostAttribute(span, costUSD)

	slog.Info("request completed",
		"request_id", requestID,
		"trace_id", traceID,
		"tenant_id", tenant.ID,
		"provider", usedProvider.ID(),
		"model", req.Model,
		"latency_ms", latency,
		"cost_usd", costUSD,
		"tokens_input", resp.Usage.PromptTokens,
		"tokens_output", resp.Usage.CompletionTokens,
	)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleStreamingResponse(w http.ResponseWriter, r *http.Request, provider router.Provider, req domain.ChatRequest, tenant *domain.Tenant, requestID string, traceID string, start time.Time) {
	ctx := r.Context()

	ctx, span := telemetry.StartSpan(ctx, "chat.completions.stream")
	defer span.End()

	metrics.ActiveStreams.Inc()
	defer metrics.ActiveStreams.Dec()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-ID", requestID)

	chunks, errs := provider.ChatCompletionStream(ctx, req)

	for {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				latency := time.Since(start).Milliseconds()
				gatewayData := domain.Gateway{
					Provider:  provider.ID(),
					LatencyMs: latency,
					CostUSD:   0,
					CacheHit:  false,
					RequestID: requestID,
					TraceID:   traceID,
				}
				gatewayJSON, _ := json.Marshal(map[string]interface{}{"x_gateway": gatewayData})
				w.Write([]byte("data: " + string(gatewayJSON) + "\n\n"))
				w.Write([]byte("data: [DONE]\n\n"))
				flusher.Flush()

				metrics.RecordRequest(tenant.ID, provider.ID(), req.Model, "success", float64(latency)/1000)
				telemetry.AddRequestAttributes(span, tenant.ID, provider.ID(), req.Model, requestID)

				slog.Info("streaming request completed",
					"request_id", requestID,
					"trace_id", traceID,
					"tenant_id", tenant.ID,
					"provider", provider.ID(),
					"model", req.Model,
					"latency_ms", latency,
				)
				h.router.RecordSuccess(provider.ID())
				return
			}

			data, _ := json.Marshal(chunk)
			w.Write([]byte("data: " + string(data) + "\n\n"))
			flusher.Flush()

		case err, ok := <-errs:
			if ok && err != nil {
				slog.Error("streaming error", "error", err, "request_id", requestID)
				metrics.RecordProviderError(provider.ID(), "stream_error")
				h.router.RecordFailure(provider.ID())
				telemetry.AddErrorAttribute(span, err)
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (h *Handler) handleListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var allModels []domain.Model

	for _, providerID := range h.router.ListProviders() {
		provider, ok := h.router.GetProvider(providerID)
		if !ok {
			continue
		}

		models, err := provider.Models(ctx)
		if err != nil {
			slog.Warn("failed to get models from provider", "provider", providerID, "error", err)
			continue
		}

		allModels = append(allModels, models...)
	}

	resp := domain.ModelsResponse{
		Object: "list",
		Data:   allModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	apiKey := extractAPIKey(r)
	if apiKey == "" {
		writeError(w, http.StatusUnauthorized, "missing API key")
		return
	}

	tenant, err := h.tenantRepo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	if h.costTracker == nil {
		writeError(w, http.StatusNotImplemented, "usage tracking not enabled")
		return
	}

	startOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)
	records, err := h.costTracker.GetTenantUsage(ctx, tenant.ID, startOfMonth)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get usage")
		return
	}

	totalCost, _ := h.costTracker.GetTenantTotalCost(ctx, tenant.ID, startOfMonth)

	resp := map[string]interface{}{
		"tenant_id":       tenant.ID,
		"period_start":    startOfMonth.Format(time.RFC3339),
		"period_end":      time.Now().Format(time.RFC3339),
		"total_cost_usd":  totalCost,
		"budget_usd":      tenant.BudgetUSD,
		"budget_used_pct": 0.0,
		"request_count":   len(records),
	}

	if tenant.BudgetUSD > 0 {
		resp["budget_used_pct"] = (totalCost / tenant.BudgetUSD) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	providers := make(map[string]string)
	allHealthy := true

	for _, providerID := range h.router.ListProviders() {
		provider, ok := h.router.GetProvider(providerID)
		if !ok {
			continue
		}

		if err := provider.HealthCheck(ctx); err != nil {
			providers[providerID] = "unhealthy"
			allHealthy = false
		} else {
			providers[providerID] = "ok"
		}
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "degraded"
	}

	resp := map[string]interface{}{
		"status":           status,
		"version":          "0.5.0",
		"providers":        providers,
		"circuit_breakers": h.router.CircuitBreakerStates(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleHealthReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "error",
			"code":    status,
		},
	})
}
