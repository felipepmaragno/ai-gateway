# ADR-008: Multi-Provider Fallback Strategy

## Status

Accepted

## Context

LLM providers can fail due to:
- Rate limiting (429)
- Service outages
- Network issues
- Model unavailability

A single-provider architecture means any provider failure causes complete service disruption.

## Decision

Implement **automatic fallback** across multiple providers with configurable priority order.

## Rationale

- **High availability** — Service continues even when primary provider fails
- **Cost optimization** — Can route to cheaper providers when appropriate
- **Flexibility** — Different tenants can have different provider preferences
- **Graceful degradation** — Fallback to less capable but available provider

## Implementation

### Provider Selection Flow

```
1. Use tenant's default_provider (if set and healthy)
2. Try fallback_providers in order (if configured)
3. Try any healthy provider
4. Return error if all providers fail
```

### Tenant Configuration

```go
type Tenant struct {
    DefaultProvider   string   // Primary choice
    FallbackProviders []string // Ordered fallback list
}
```

### Router Logic

```go
func (r *Router) SelectProvider(tenant *Tenant, model string) (Provider, error) {
    // 1. Try default provider
    if p := r.getHealthyProvider(tenant.DefaultProvider); p != nil {
        return p, nil
    }
    
    // 2. Try fallback providers in order
    for _, name := range tenant.FallbackProviders {
        if p := r.getHealthyProvider(name); p != nil {
            return p, nil
        }
    }
    
    // 3. Try any healthy provider
    for name, p := range r.providers {
        if r.isHealthy(name) {
            return p, nil
        }
    }
    
    return nil, ErrNoProvidersAvailable
}
```

### Health Checking

Provider health is determined by:
- Circuit breaker state (closed = healthy)
- Recent error rate
- Explicit health check endpoint (if available)

## Consequences

### Positive
- Increased availability
- Automatic recovery from provider failures
- Per-tenant customization
- Transparent to API consumers

### Negative
- Response quality may vary between providers
- Cost implications of fallback (e.g., GPT-4 → Claude)
- Model compatibility issues (not all models on all providers)

## Model Mapping

When falling back, the router attempts to map models:

| Requested Model | OpenAI | Anthropic | Ollama |
|-----------------|--------|-----------|--------|
| gpt-4 | gpt-4 | claude-3-opus | - |
| gpt-3.5-turbo | gpt-3.5-turbo | claude-3-haiku | llama3 |
| claude-3-opus | - | claude-3-opus | - |

If no mapping exists, the original model name is passed through.

## Alternatives Considered

### Client-Side Fallback
- Requires client changes
- Duplicates logic across consumers
- Harder to manage centrally

### Load Balancer Level
- No application-level health awareness
- Cannot consider model compatibility
- Less flexible routing rules
