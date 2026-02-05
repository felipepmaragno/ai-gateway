// Package router provides LLM provider selection and load balancing.
// It selects the best available provider based on health status,
// tenant preferences, and fallback chains for resilience.
package router

import (
	"context"
	"log/slog"

	"github.com/felipepmaragno/ai-gateway/internal/circuitbreaker"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

// Provider defines the interface that all LLM providers must implement.
// Each provider handles communication with a specific LLM service (OpenAI, Anthropic, etc.).
type Provider interface {
	ID() string
	ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error)
	Models(ctx context.Context) ([]domain.Model, error)
	HealthCheck(ctx context.Context) error
}

// Router manages provider selection with health-aware routing and automatic fallback.
type Router struct {
	providers       map[string]Provider
	defaultProvider string
	fallbackOrder   []string
	cbManager       *circuitbreaker.Manager
}

type Config struct {
	Providers       map[string]Provider
	DefaultProvider string
	FallbackOrder   []string
	CBConfig        circuitbreaker.Config
}

func New(providers map[string]Provider, defaultProvider string) *Router {
	fallbackOrder := make([]string, 0, len(providers))
	for id := range providers {
		fallbackOrder = append(fallbackOrder, id)
	}

	return &Router{
		providers:       providers,
		defaultProvider: defaultProvider,
		fallbackOrder:   fallbackOrder,
		cbManager:       circuitbreaker.NewManager(circuitbreaker.DefaultConfig()),
	}
}

func NewWithConfig(cfg Config) *Router {
	fallbackOrder := cfg.FallbackOrder
	if len(fallbackOrder) == 0 {
		fallbackOrder = make([]string, 0, len(cfg.Providers))
		for id := range cfg.Providers {
			fallbackOrder = append(fallbackOrder, id)
		}
	}

	return &Router{
		providers:       cfg.Providers,
		defaultProvider: cfg.DefaultProvider,
		fallbackOrder:   fallbackOrder,
		cbManager:       circuitbreaker.NewManager(cfg.CBConfig),
	}
}

func (r *Router) SelectProvider(ctx context.Context, providerHint string, model string) (Provider, error) {
	if providerHint != "" {
		if p, ok := r.providers[providerHint]; ok {
			cb := r.cbManager.Get(providerHint)
			if err := cb.Allow(); err != nil {
				slog.Warn("circuit breaker open for requested provider", "provider", providerHint)
				return nil, err
			}
			return p, nil
		}
		return nil, domain.ErrProviderNotFound
	}

	if p := r.findProviderByModel(model); p != nil {
		cb := r.cbManager.Get(p.ID())
		if cb.Allow() == nil {
			return p, nil
		}
		slog.Warn("circuit breaker open for model provider, trying fallback", "provider", p.ID())
	}

	if p, ok := r.providers[r.defaultProvider]; ok {
		cb := r.cbManager.Get(r.defaultProvider)
		if cb.Allow() == nil {
			return p, nil
		}
		slog.Warn("circuit breaker open for default provider, trying fallback", "provider", r.defaultProvider)
	}

	for _, id := range r.fallbackOrder {
		cb := r.cbManager.Get(id)
		if cb.Allow() == nil {
			if p, ok := r.providers[id]; ok {
				slog.Info("using fallback provider", "provider", id)
				return p, nil
			}
		}
	}

	return nil, domain.ErrProviderNotFound
}

func (r *Router) SelectProviderWithFallback(ctx context.Context, providerHint string, model string) ([]Provider, error) {
	var providers []Provider

	primary, _ := r.SelectProvider(ctx, providerHint, model)
	if primary != nil {
		providers = append(providers, primary)
	}

	for _, id := range r.fallbackOrder {
		if primary != nil && id == primary.ID() {
			continue
		}
		cb := r.cbManager.Get(id)
		if cb.Allow() == nil {
			if p, ok := r.providers[id]; ok {
				providers = append(providers, p)
			}
		}
	}

	if len(providers) == 0 {
		return nil, domain.ErrProviderNotFound
	}

	return providers, nil
}

func (r *Router) RecordSuccess(providerID string) {
	r.cbManager.Get(providerID).RecordSuccess()
}

func (r *Router) RecordFailure(providerID string) {
	r.cbManager.Get(providerID).RecordFailure()
}

func (r *Router) CircuitBreakerStates() map[string]string {
	return r.cbManager.States()
}

func (r *Router) findProviderByModel(model string) Provider {
	modelProviderMap := map[string]string{
		"gpt-4":         "openai",
		"gpt-4-turbo":   "openai",
		"gpt-3.5-turbo": "openai",
		"claude-3":      "anthropic",
	}

	if providerID, ok := modelProviderMap[model]; ok {
		if p, ok := r.providers[providerID]; ok {
			return p
		}
	}

	return nil
}

func (r *Router) GetProvider(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

func (r *Router) ListProviders() []string {
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}
