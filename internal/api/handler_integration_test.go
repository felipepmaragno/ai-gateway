//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/api"
	"github.com/felipepmaragno/ai-gateway/internal/budget"
	"github.com/felipepmaragno/ai-gateway/internal/cache"
	"github.com/felipepmaragno/ai-gateway/internal/cost"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/felipepmaragno/ai-gateway/internal/ratelimit"
	"github.com/felipepmaragno/ai-gateway/internal/repository"
	"github.com/felipepmaragno/ai-gateway/internal/router"
)

type mockProvider struct {
	id     string
	models []domain.Model
}

func (m *mockProvider) ID() string { return m.id }

func (m *mockProvider) ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	return &domain.ChatResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []domain.Choice{
			{
				Index:        0,
				Message:      &domain.Message{Role: "assistant", Content: "Hello!"},
				FinishReason: "stop",
			},
		},
		Usage: domain.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (m *mockProvider) ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	chunks := make(chan domain.StreamChunk)
	errs := make(chan error)
	close(chunks)
	close(errs)
	return chunks, errs
}

func (m *mockProvider) Models(ctx context.Context) ([]domain.Model, error) {
	return m.models, nil
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func setupTestHandler(t *testing.T) *api.Handler {
	t.Helper()

	tenantRepo := repository.NewInMemoryTenantRepository()
	rateLimiter := ratelimit.NewInMemoryRateLimiter()
	responseCache := cache.NewInMemoryCache()
	costTracker := cost.NewInMemoryTracker()
	budgetMonitor := budget.NewMonitor(costTracker, budget.DefaultThresholds())

	providers := map[string]router.Provider{
		"mock": &mockProvider{
			id: "mock",
			models: []domain.Model{
				{ID: "test-model", Object: "model", OwnedBy: "test", Provider: "mock"},
			},
		},
	}
	providerRouter := router.New(providers, "mock")

	return api.NewHandler(api.HandlerConfig{
		TenantRepo:    tenantRepo,
		RateLimiter:   rateLimiter,
		Router:        providerRouter,
		Cache:         responseCache,
		CacheTTL:      5 * time.Minute,
		CostTracker:   costTracker,
		BudgetMonitor: budgetMonitor,
	})
}

func TestHealthEndpoint(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", resp["status"])
	}
}

func TestChatCompletionUnauthorized(t *testing.T) {
	handler := setupTestHandler(t)

	body := `{"model": "test-model", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestChatCompletionSuccess(t *testing.T) {
	handler := setupTestHandler(t)

	body := `{"model": "test-model", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer gw-default-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp domain.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Error("expected at least one choice")
	}

	if resp.Gateway == nil {
		t.Error("expected gateway metadata")
	}
}

func TestModelsEndpoint(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer gw-default-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp domain.ModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) == 0 {
		t.Error("expected at least one model")
	}
}

func TestRateLimiting(t *testing.T) {
	tenantRepo := repository.NewInMemoryTenantRepository()
	rateLimiter := ratelimit.NewInMemoryRateLimiter()
	responseCache := cache.NewInMemoryCache()
	costTracker := cost.NewInMemoryTracker()
	budgetMonitor := budget.NewMonitor(costTracker, budget.DefaultThresholds())

	providers := map[string]router.Provider{
		"mock": &mockProvider{id: "mock"},
	}
	providerRouter := router.New(providers, "mock")

	handler := api.NewHandler(api.HandlerConfig{
		TenantRepo:    tenantRepo,
		RateLimiter:   rateLimiter,
		Router:        providerRouter,
		Cache:         responseCache,
		CacheTTL:      5 * time.Minute,
		CostTracker:   costTracker,
		BudgetMonitor: budgetMonitor,
	})

	body := `{"model": "test-model", "messages": [{"role": "user", "content": "Hi"}]}`

	for i := 0; i < 150; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer gw-default-key")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if i >= 100 && w.Code != http.StatusTooManyRequests {
			t.Errorf("request %d: expected 429 after rate limit, got %d", i, w.Code)
			break
		}
	}
}

func TestCacheHit(t *testing.T) {
	handler := setupTestHandler(t)

	body := `{"model": "test-model", "messages": [{"role": "user", "content": "Cache test"}], "temperature": 0}`

	req1 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer gw-default-key")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	var resp1 domain.ChatResponse
	json.NewDecoder(w1.Body).Decode(&resp1)

	if resp1.Gateway != nil && resp1.Gateway.CacheHit {
		t.Error("first request should not be a cache hit")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer gw-default-key")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	var resp2 domain.ChatResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if resp2.Gateway == nil || !resp2.Gateway.CacheHit {
		t.Error("second request should be a cache hit")
	}
}
