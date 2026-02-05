# Router Package

Provider selection and load balancing for LLM providers.

## Overview

The router selects the best available provider for each request, considering:
- Provider health status
- Tenant preferences (default provider, allowed models)
- Fallback chain for resilience

## Architecture

```
Request → Provider Hint? → Health Check → Select Provider
              ↓                              ↓
         Use Hint              Fallback to next healthy provider
```

## Key Components

### Router Interface

```go
type Router interface {
    SelectProvider(ctx context.Context, hint string, model string) (Provider, error)
}
```

### Provider Interface

All LLM providers implement this interface:

```go
type Provider interface {
    ID() string
    ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, error)
    Models(ctx context.Context) ([]domain.Model, error)
    HealthCheck(ctx context.Context) error
}
```

## Provider Selection Logic

1. **Explicit hint**: If request specifies `X-Provider` header, use that provider
2. **Tenant default**: Use tenant's configured default provider
3. **First healthy**: Select first healthy provider from the pool
4. **Fallback chain**: If primary fails, try fallback providers in order

## Health Checks

Providers are checked periodically:
- Healthy providers are preferred
- Unhealthy providers are skipped unless no alternatives exist
- Circuit breaker prevents cascading failures

## Supported Providers

| Provider | Models | Streaming |
|----------|--------|-----------|
| OpenAI | GPT-4, GPT-3.5 | ✅ |
| Anthropic | Claude 3.x | ✅ |
| Ollama | Local models | ✅ |
| AWS Bedrock | Claude, Titan | ✅ |

## Usage Example

```go
router := router.New(providers, router.Config{
    DefaultProvider: "openai",
    FallbackOrder:   []string{"anthropic", "ollama"},
})

provider, err := router.SelectProvider(ctx, "", "gpt-4")
if err != nil {
    // No healthy provider available
}

resp, err := provider.ChatCompletion(ctx, req)
```

## Dependencies

- `internal/domain` - Request/response types
- `internal/circuitbreaker` - Failure protection
