package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/cache"
	"github.com/felipepmaragno/ai-gateway/internal/cost"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/felipepmaragno/ai-gateway/internal/ratelimit"
	"github.com/felipepmaragno/ai-gateway/internal/router"
)

// =============================================================================
// Mock Implementations (Interface-Based Mocking Pattern)
// =============================================================================

// MockTenantRepository implements repository.TenantRepository for testing
type MockTenantRepository struct {
	GetByAPIKeyFunc func(ctx context.Context, apiKey string) (*domain.Tenant, error)
	GetByIDFunc     func(ctx context.Context, id string) (*domain.Tenant, error)
	CreateFunc      func(ctx context.Context, tenant *domain.Tenant) error
	UpdateFunc      func(ctx context.Context, tenant *domain.Tenant) error
	DeleteFunc      func(ctx context.Context, id string) error
	ListFunc        func(ctx context.Context) ([]*domain.Tenant, error)
}

func (m *MockTenantRepository) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error) {
	if m.GetByAPIKeyFunc != nil {
		return m.GetByAPIKeyFunc(ctx, apiKey)
	}
	return nil, errors.New("not implemented")
}

func (m *MockTenantRepository) GetByID(ctx context.Context, id string) (*domain.Tenant, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *MockTenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tenant)
	}
	return nil
}

func (m *MockTenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, tenant)
	}
	return nil
}

func (m *MockTenantRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockTenantRepository) List(ctx context.Context) ([]*domain.Tenant, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

// MockRateLimiter implements ratelimit.RateLimiter for testing
type MockRateLimiter struct {
	AllowFunc func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error)
}

func (m *MockRateLimiter) Allow(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
	if m.AllowFunc != nil {
		return m.AllowFunc(ctx, tenantID, limit)
	}
	return true, limit - 1, time.Now().Add(time.Minute), nil
}

// MockCache implements cache.Cache for testing
type MockCache struct {
	GetFunc func(ctx context.Context, key string) (*domain.ChatResponse, bool)
	SetFunc func(ctx context.Context, key string, resp *domain.ChatResponse, ttl time.Duration) error
}

func (m *MockCache) Get(ctx context.Context, key string) (*domain.ChatResponse, bool) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return nil, false
}

func (m *MockCache) Set(ctx context.Context, key string, resp *domain.ChatResponse, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, resp, ttl)
	}
	return nil
}

// MockProvider implements router.Provider for testing
type MockProvider struct {
	IDValue                   string
	ChatCompletionFunc        func(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error)
	ChatCompletionStreamFunc  func(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error)
	ModelsFunc                func(ctx context.Context) ([]domain.Model, error)
	HealthCheckFunc           func(ctx context.Context) error
}

func (m *MockProvider) ID() string {
	return m.IDValue
}

func (m *MockProvider) ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	if m.ChatCompletionFunc != nil {
		return m.ChatCompletionFunc(ctx, req)
	}
	return &domain.ChatResponse{
		ID:     "test-response",
		Object: "chat.completion",
		Model:  req.Model,
		Usage:  domain.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}

func (m *MockProvider) ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	if m.ChatCompletionStreamFunc != nil {
		return m.ChatCompletionStreamFunc(ctx, req)
	}
	chunks := make(chan domain.StreamChunk)
	errs := make(chan error)
	close(chunks)
	return chunks, errs
}

func (m *MockProvider) Models(ctx context.Context) ([]domain.Model, error) {
	if m.ModelsFunc != nil {
		return m.ModelsFunc(ctx)
	}
	return []domain.Model{{ID: "gpt-4", Object: "model"}}, nil
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	return nil
}

// MockCostTracker implements cost.Tracker for testing
type MockCostTracker struct {
	RecordFunc            func(ctx context.Context, record cost.UsageRecord) error
	GetTenantTotalCostFunc func(ctx context.Context, tenantID string, since time.Time) (float64, error)
	GetTenantUsageFunc    func(ctx context.Context, tenantID string, since time.Time) ([]cost.UsageRecord, error)
}

func (m *MockCostTracker) Record(ctx context.Context, record cost.UsageRecord) error {
	if m.RecordFunc != nil {
		return m.RecordFunc(ctx, record)
	}
	return nil
}

func (m *MockCostTracker) GetTenantTotalCost(ctx context.Context, tenantID string, since time.Time) (float64, error) {
	if m.GetTenantTotalCostFunc != nil {
		return m.GetTenantTotalCostFunc(ctx, tenantID, since)
	}
	return 0, nil
}

func (m *MockCostTracker) GetTenantUsage(ctx context.Context, tenantID string, since time.Time) ([]cost.UsageRecord, error) {
	if m.GetTenantUsageFunc != nil {
		return m.GetTenantUsageFunc(ctx, tenantID, since)
	}
	return nil, nil
}

// =============================================================================
// Test Helpers
// =============================================================================

func setupTestHandler(t *testing.T) (*Handler, *MockTenantRepository, *MockRateLimiter, *MockCache, *MockProvider) {
	t.Helper()

	tenantRepo := &MockTenantRepository{}
	rateLimiter := &MockRateLimiter{}
	mockCache := &MockCache{}
	mockProvider := &MockProvider{IDValue: "openai"}

	providers := map[string]router.Provider{
		"openai": mockProvider,
	}
	r := router.New(providers, "openai")

	handler := NewHandler(HandlerConfig{
		TenantRepo:  tenantRepo,
		RateLimiter: rateLimiter,
		Router:      r,
		Cache:       mockCache,
		CacheTTL:    5 * time.Minute,
	})

	return handler, tenantRepo, rateLimiter, mockCache, mockProvider
}

func createTestTenant() *domain.Tenant {
	return &domain.Tenant{
		ID:           "tenant-123",
		Name:         "Test Tenant",
		APIKey:       "sk-test-key",
		RateLimitRPM: 100,
		BudgetUSD:    1000.0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func createChatRequest(model string, stream bool) domain.ChatRequest {
	return domain.ChatRequest{
		Model: model,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello, world!"},
		},
		Stream: stream,
	}
}

// =============================================================================
// Table-Driven Tests for Chat Completions
// =============================================================================

func TestHandleChatCompletions(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*MockTenantRepository, *MockRateLimiter, *MockCache, *MockProvider)
		request        func() *http.Request
		wantStatus     int
		wantBodyContains string
	}{
		{
			name: "successful request",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return createTestTenant(), nil
				}
				rl.AllowFunc = func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
					return true, 99, time.Now().Add(time.Minute), nil
				}
				c.GetFunc = func(ctx context.Context, key string) (*domain.ChatResponse, bool) {
					return nil, false
				}
				p.ChatCompletionFunc = func(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
					return &domain.ChatResponse{
						ID:     "resp-123",
						Object: "chat.completion",
						Model:  req.Model,
						Usage:  domain.Usage{PromptTokens: 10, CompletionTokens: 20},
					}, nil
				}
			},
			request: func() *http.Request {
				body, _ := json.Marshal(createChatRequest("gpt-4", false))
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer sk-test-key")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:     http.StatusOK,
			wantBodyContains: "chat.completion",
		},
		{
			name: "missing API key",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				// No setup needed - should fail before hitting mocks
			},
			request: func() *http.Request {
				body, _ := json.Marshal(createChatRequest("gpt-4", false))
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				// No Authorization header
				return req
			},
			wantStatus:     http.StatusUnauthorized,
			wantBodyContains: "missing API key",
		},
		{
			name: "invalid API key",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return nil, errors.New("tenant not found")
				}
			},
			request: func() *http.Request {
				body, _ := json.Marshal(createChatRequest("gpt-4", false))
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer invalid-key")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:     http.StatusUnauthorized,
			wantBodyContains: "invalid API key",
		},
		{
			name: "rate limit exceeded",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return createTestTenant(), nil
				}
				rl.AllowFunc = func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
					return false, 0, time.Now().Add(time.Minute), nil
				}
			},
			request: func() *http.Request {
				body, _ := json.Marshal(createChatRequest("gpt-4", false))
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer sk-test-key")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:     http.StatusTooManyRequests,
			wantBodyContains: "rate limit exceeded",
		},
		{
			name: "invalid request body",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return createTestTenant(), nil
				}
				rl.AllowFunc = func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
					return true, 99, time.Now().Add(time.Minute), nil
				}
			},
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte("invalid json")))
				req.Header.Set("Authorization", "Bearer sk-test-key")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:     http.StatusBadRequest,
			wantBodyContains: "invalid request body",
		},
		{
			name: "cache hit",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return createTestTenant(), nil
				}
				rl.AllowFunc = func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
					return true, 99, time.Now().Add(time.Minute), nil
				}
				c.GetFunc = func(ctx context.Context, key string) (*domain.ChatResponse, bool) {
					return &domain.ChatResponse{
						ID:     "cached-response",
						Object: "chat.completion",
						Model:  "gpt-4",
					}, true
				}
			},
			request: func() *http.Request {
				body, _ := json.Marshal(createChatRequest("gpt-4", false))
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer sk-test-key")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:     http.StatusOK,
			wantBodyContains: "cached-response",
		},
		{
			name: "provider error with fallback",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return createTestTenant(), nil
				}
				rl.AllowFunc = func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
					return true, 99, time.Now().Add(time.Minute), nil
				}
				c.GetFunc = func(ctx context.Context, key string) (*domain.ChatResponse, bool) {
					return nil, false
				}
				callCount := 0
				p.ChatCompletionFunc = func(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
					callCount++
					if callCount == 1 {
						return nil, errors.New("provider temporarily unavailable")
					}
					return &domain.ChatResponse{
						ID:     "fallback-response",
						Object: "chat.completion",
						Model:  req.Model,
						Usage:  domain.Usage{PromptTokens: 10, CompletionTokens: 20},
					}, nil
				}
			},
			request: func() *http.Request {
				body, _ := json.Marshal(createChatRequest("gpt-4", false))
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer sk-test-key")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:     http.StatusBadGateway,
			wantBodyContains: "all providers failed",
		},
		{
			name: "rate limiter error",
			setupMocks: func(repo *MockTenantRepository, rl *MockRateLimiter, c *MockCache, p *MockProvider) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return createTestTenant(), nil
				}
				rl.AllowFunc = func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
					return false, 0, time.Time{}, errors.New("redis connection failed")
				}
			},
			request: func() *http.Request {
				body, _ := json.Marshal(createChatRequest("gpt-4", false))
				req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer sk-test-key")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantStatus:     http.StatusInternalServerError,
			wantBodyContains: "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, repo, rl, c, p := setupTestHandler(t)
			tt.setupMocks(repo, rl, c, p)

			req := tt.request()
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			if tt.wantBodyContains != "" && !bytes.Contains(rr.Body.Bytes(), []byte(tt.wantBodyContains)) {
				t.Errorf("body = %q, want to contain %q", rr.Body.String(), tt.wantBodyContains)
			}
		})
	}
}

// =============================================================================
// Tests for Health Endpoints
// =============================================================================

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"health live", "/health/live", http.StatusOK},
		{"health ready", "/health/ready", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _, _, _, _ := setupTestHandler(t)

			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	tests := []struct {
		name           string
		setupProvider  func(*MockProvider)
		wantStatus     int
		wantBodyContains string
	}{
		{
			name: "all providers healthy",
			setupProvider: func(p *MockProvider) {
				p.HealthCheckFunc = func(ctx context.Context) error {
					return nil
				}
			},
			wantStatus:     http.StatusOK,
			wantBodyContains: "healthy",
		},
		{
			name: "provider unhealthy - degraded",
			setupProvider: func(p *MockProvider) {
				p.HealthCheckFunc = func(ctx context.Context) error {
					return errors.New("connection refused")
				}
			},
			wantStatus:     http.StatusOK,
			wantBodyContains: "degraded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _, _, _, provider := setupTestHandler(t)
			tt.setupProvider(provider)

			req := httptest.NewRequest("GET", "/health", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			if !bytes.Contains(rr.Body.Bytes(), []byte(tt.wantBodyContains)) {
				t.Errorf("body = %q, want to contain %q", rr.Body.String(), tt.wantBodyContains)
			}
		})
	}
}

// =============================================================================
// Tests for List Models
// =============================================================================

func TestHandleListModels(t *testing.T) {
	tests := []struct {
		name          string
		setupProvider func(*MockProvider)
		wantStatus    int
		wantModels    int
	}{
		{
			name: "returns models from provider",
			setupProvider: func(p *MockProvider) {
				p.ModelsFunc = func(ctx context.Context) ([]domain.Model, error) {
					return []domain.Model{
						{ID: "gpt-4", Object: "model"},
						{ID: "gpt-3.5-turbo", Object: "model"},
					}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantModels: 2,
		},
		{
			name: "provider error - returns empty",
			setupProvider: func(p *MockProvider) {
				p.ModelsFunc = func(ctx context.Context) ([]domain.Model, error) {
					return nil, errors.New("provider unavailable")
				}
			},
			wantStatus: http.StatusOK,
			wantModels: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _, _, _, provider := setupTestHandler(t)
			tt.setupProvider(provider)

			req := httptest.NewRequest("GET", "/v1/models", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			var resp domain.ModelsResponse
			json.Unmarshal(rr.Body.Bytes(), &resp)

			if len(resp.Data) != tt.wantModels {
				t.Errorf("models count = %d, want %d", len(resp.Data), tt.wantModels)
			}
		})
	}
}

// =============================================================================
// Tests for Usage Endpoint
// =============================================================================

func TestHandleUsage(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*MockTenantRepository, *MockCostTracker)
		apiKey         string
		wantStatus     int
		wantBodyContains string
	}{
		{
			name: "returns usage data",
			setupMocks: func(repo *MockTenantRepository, tracker *MockCostTracker) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return createTestTenant(), nil
				}
				tracker.GetTenantUsageFunc = func(ctx context.Context, tenantID string, since time.Time) ([]cost.UsageRecord, error) {
					return []cost.UsageRecord{
						{TenantID: tenantID, CostUSD: 0.05},
						{TenantID: tenantID, CostUSD: 0.03},
					}, nil
				}
				tracker.GetTenantTotalCostFunc = func(ctx context.Context, tenantID string, since time.Time) (float64, error) {
					return 0.08, nil
				}
			},
			apiKey:         "sk-test-key",
			wantStatus:     http.StatusOK,
			wantBodyContains: "total_cost_usd",
		},
		{
			name: "missing API key",
			setupMocks: func(repo *MockTenantRepository, tracker *MockCostTracker) {
				// No setup needed
			},
			apiKey:         "",
			wantStatus:     http.StatusUnauthorized,
			wantBodyContains: "missing API key",
		},
		{
			name: "invalid API key",
			setupMocks: func(repo *MockTenantRepository, tracker *MockCostTracker) {
				repo.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
					return nil, errors.New("not found")
				}
			},
			apiKey:         "invalid-key",
			wantStatus:     http.StatusUnauthorized,
			wantBodyContains: "invalid API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantRepo := &MockTenantRepository{}
			costTracker := &MockCostTracker{}
			tt.setupMocks(tenantRepo, costTracker)

			mockProvider := &MockProvider{IDValue: "openai"}
			providers := map[string]router.Provider{"openai": mockProvider}
			r := router.New(providers, "openai")

			handler := NewHandler(HandlerConfig{
				TenantRepo:  tenantRepo,
				RateLimiter: ratelimit.NewInMemoryRateLimiter(),
				Router:      r,
				Cache:       cache.NewInMemoryCache(),
				CostTracker: costTracker,
			})

			req := httptest.NewRequest("GET", "/v1/usage", nil)
			if tt.apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+tt.apiKey)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			if !bytes.Contains(rr.Body.Bytes(), []byte(tt.wantBodyContains)) {
				t.Errorf("body = %q, want to contain %q", rr.Body.String(), tt.wantBodyContains)
			}
		})
	}
}

// =============================================================================
// Tests for Helper Functions
// =============================================================================

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer token", "Bearer sk-test-123", "sk-test-123"},
		{"no bearer prefix", "sk-test-123", ""},
		{"empty header", "", ""},
		{"basic auth", "Basic dXNlcjpwYXNz", ""},
		{"bearer with extra spaces", "Bearer  sk-test", " sk-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := extractAPIKey(req)
			if got != tt.want {
				t.Errorf("extractAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		message    string
		wantStatus int
	}{
		{"bad request", http.StatusBadRequest, "invalid input", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized, "missing token", http.StatusUnauthorized},
		{"internal error", http.StatusInternalServerError, "something went wrong", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			writeError(rr, tt.status, tt.message)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			contentType := rr.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", contentType)
			}

			var resp map[string]interface{}
			json.Unmarshal(rr.Body.Bytes(), &resp)

			errObj, ok := resp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("response should contain error object")
			}

			if errObj["message"] != tt.message {
				t.Errorf("error message = %q, want %q", errObj["message"], tt.message)
			}
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkHandleChatCompletions(b *testing.B) {
	tenantRepo := &MockTenantRepository{
		GetByAPIKeyFunc: func(ctx context.Context, apiKey string) (*domain.Tenant, error) {
			return createTestTenant(), nil
		},
	}
	rateLimiter := &MockRateLimiter{
		AllowFunc: func(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
			return true, 99, time.Now().Add(time.Minute), nil
		},
	}
	mockCache := &MockCache{
		GetFunc: func(ctx context.Context, key string) (*domain.ChatResponse, bool) {
			return nil, false
		},
	}
	mockProvider := &MockProvider{
		IDValue: "openai",
		ChatCompletionFunc: func(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
			return &domain.ChatResponse{
				ID:     "resp-123",
				Object: "chat.completion",
				Model:  req.Model,
				Usage:  domain.Usage{PromptTokens: 10, CompletionTokens: 20},
			}, nil
		},
	}

	providers := map[string]router.Provider{"openai": mockProvider}
	r := router.New(providers, "openai")

	handler := NewHandler(HandlerConfig{
		TenantRepo:  tenantRepo,
		RateLimiter: rateLimiter,
		Router:      r,
		Cache:       mockCache,
	})

	body, _ := json.Marshal(createChatRequest("gpt-4", false))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer sk-test-key")
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
