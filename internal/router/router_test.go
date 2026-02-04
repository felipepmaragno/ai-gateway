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
