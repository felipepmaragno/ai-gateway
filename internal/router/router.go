package router

import (
	"context"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type Provider interface {
	ID() string
	ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error)
	Models(ctx context.Context) ([]domain.Model, error)
	HealthCheck(ctx context.Context) error
}

type Router struct {
	providers       map[string]Provider
	defaultProvider string
}

func New(providers map[string]Provider, defaultProvider string) *Router {
	return &Router{
		providers:       providers,
		defaultProvider: defaultProvider,
	}
}

func (r *Router) SelectProvider(ctx context.Context, providerHint string, model string) (Provider, error) {
	if providerHint != "" {
		if p, ok := r.providers[providerHint]; ok {
			return p, nil
		}
		return nil, domain.ErrProviderNotFound
	}

	if p := r.findProviderByModel(model); p != nil {
		return p, nil
	}

	if p, ok := r.providers[r.defaultProvider]; ok {
		return p, nil
	}

	for _, p := range r.providers {
		return p, nil
	}

	return nil, domain.ErrProviderNotFound
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
