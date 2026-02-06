package router

import (
	"context"
	"testing"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type mockProvider struct {
	id string
}

func (m *mockProvider) ID() string { return m.id }
func (m *mockProvider) ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	return nil, nil
}
func (m *mockProvider) ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	return nil, nil
}
func (m *mockProvider) Models(ctx context.Context) ([]domain.Model, error) { return nil, nil }
func (m *mockProvider) HealthCheck(ctx context.Context) error              { return nil }

func TestRouter_SelectProvider_WithHint(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
		"ollama": &mockProvider{id: "ollama"},
	}

	r := New(providers, "ollama")

	p, err := r.SelectProvider(context.Background(), "openai", "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.ID() != "openai" {
		t.Errorf("expected openai, got %s", p.ID())
	}
}

func TestRouter_SelectProvider_WithDefault(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
		"ollama": &mockProvider{id: "ollama"},
	}

	r := New(providers, "ollama")

	p, err := r.SelectProvider(context.Background(), "", "some-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.ID() != "ollama" {
		t.Errorf("expected ollama, got %s", p.ID())
	}
}

func TestRouter_SelectProvider_ByModel(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
		"ollama": &mockProvider{id: "ollama"},
	}

	r := New(providers, "ollama")

	p, err := r.SelectProvider(context.Background(), "", "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.ID() != "openai" {
		t.Errorf("expected openai for gpt-4, got %s", p.ID())
	}
}

func TestRouter_SelectProvider_NotFound(t *testing.T) {
	providers := map[string]Provider{
		"ollama": &mockProvider{id: "ollama"},
	}

	r := New(providers, "ollama")

	_, err := r.SelectProvider(context.Background(), "nonexistent", "model")
	if err != domain.ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestRouter_NewWithConfig(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
		"ollama": &mockProvider{id: "ollama"},
	}

	cfg := Config{
		Providers:       providers,
		DefaultProvider: "openai",
		FallbackOrder:   []string{"openai", "ollama"},
	}

	r := NewWithConfig(cfg)

	if r.defaultProvider != "openai" {
		t.Errorf("defaultProvider = %v, want openai", r.defaultProvider)
	}
	if len(r.fallbackOrder) != 2 {
		t.Errorf("fallbackOrder length = %d, want 2", len(r.fallbackOrder))
	}
}

func TestRouter_NewWithConfig_NoFallbackOrder(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
	}

	cfg := Config{
		Providers:       providers,
		DefaultProvider: "openai",
	}

	r := NewWithConfig(cfg)

	if len(r.fallbackOrder) != 1 {
		t.Errorf("fallbackOrder should be auto-generated, got length %d", len(r.fallbackOrder))
	}
}

func TestRouter_SelectProviderWithFallback(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
		"ollama": &mockProvider{id: "ollama"},
	}

	r := New(providers, "ollama")

	providerList, err := r.SelectProviderWithFallback(context.Background(), "", "gpt-4")
	if err != nil {
		t.Fatalf("SelectProviderWithFallback() error = %v", err)
	}

	if len(providerList) < 1 {
		t.Error("should return at least one provider")
	}
}

func TestRouter_RecordSuccessAndFailure(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
	}

	r := New(providers, "openai")

	// Should not panic
	r.RecordSuccess("openai")
	r.RecordFailure("openai")
}

func TestRouter_CircuitBreakerStates(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
	}

	r := New(providers, "openai")

	// Trigger circuit breaker creation
	r.SelectProvider(context.Background(), "openai", "gpt-4")

	states := r.CircuitBreakerStates()
	if states == nil {
		t.Error("CircuitBreakerStates() should not return nil")
	}
}

func TestRouter_GetProvider(t *testing.T) {
	providers := map[string]Provider{
		"openai": &mockProvider{id: "openai"},
	}

	r := New(providers, "openai")

	p, ok := r.GetProvider("openai")
	if !ok {
		t.Error("GetProvider(openai) should return true")
	}
	if p.ID() != "openai" {
		t.Errorf("GetProvider(openai).ID() = %v, want openai", p.ID())
	}

	_, ok = r.GetProvider("nonexistent")
	if ok {
		t.Error("GetProvider(nonexistent) should return false")
	}
}

func TestRouter_ListProviders(t *testing.T) {
	providers := map[string]Provider{
		"openai":    &mockProvider{id: "openai"},
		"ollama":    &mockProvider{id: "ollama"},
		"anthropic": &mockProvider{id: "anthropic"},
	}

	r := New(providers, "openai")

	list := r.ListProviders()
	if len(list) != 3 {
		t.Errorf("ListProviders() length = %d, want 3", len(list))
	}
}

func TestRouter_FindProviderByModel_Claude(t *testing.T) {
	providers := map[string]Provider{
		"openai":    &mockProvider{id: "openai"},
		"anthropic": &mockProvider{id: "anthropic"},
	}

	r := New(providers, "openai")

	p, err := r.SelectProvider(context.Background(), "", "claude-3")
	if err != nil {
		t.Fatalf("SelectProvider() error = %v", err)
	}
	if p.ID() != "anthropic" {
		t.Errorf("claude-3 should route to anthropic, got %s", p.ID())
	}
}
