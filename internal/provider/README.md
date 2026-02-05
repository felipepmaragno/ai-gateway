# Provider Package

LLM provider implementations.

## Overview

Each subdirectory implements the `router.Provider` interface for a specific LLM service.

## Supported Providers

| Provider | Package | Features |
|----------|---------|----------|
| OpenAI | `provider/openai` | GPT-4, GPT-3.5, streaming |
| Anthropic | `provider/anthropic` | Claude 3.x, streaming |
| Ollama | `provider/ollama` | Local models, streaming |
| AWS Bedrock | `provider/bedrock` | Claude, Titan via AWS |

## Interface

All providers implement:

```go
type Provider interface {
    ID() string
    ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, error)
    Models(ctx context.Context) ([]domain.Model, error)
    HealthCheck(ctx context.Context) error
}
```

## HTTP Client

All providers use a shared HTTP client with proper timeouts:

```go
client := httputil.DefaultClient()
// Dial timeout: 10s
// TLS handshake: 10s
// Response header: 30s
// Total timeout: 120s
```

## Adding a New Provider

1. Create package under `internal/provider/<name>/`
2. Implement the `Provider` interface
3. Use `httputil.DefaultClient()` for HTTP calls
4. Handle streaming via channels
5. Register in `cmd/aigateway/main.go`

## Request/Response Mapping

Each provider maps between:
- `domain.ChatRequest` → Provider-specific request format
- Provider response → `domain.ChatResponse`

Example (OpenAI):
```go
// OpenAI uses the same format as domain types
httpReq.Body = json.Marshal(req)

// Response maps directly
var resp domain.ChatResponse
json.Decode(httpResp.Body, &resp)
```

Example (Anthropic):
```go
// Anthropic has different format
anthropicReq := toAnthropicRequest(req)  // Convert
httpReq.Body = json.Marshal(anthropicReq)

// Response needs conversion
var anthropicResp anthropicResponse
json.Decode(httpResp.Body, &anthropicResp)
return fromAnthropicResponse(anthropicResp)
```

## Error Handling

Providers should return meaningful errors:
- Connection errors → wrapped with context
- API errors → include status code and message
- Timeout errors → context deadline exceeded

## Dependencies

- `internal/domain` - Request/response types
- `internal/httputil` - HTTP client with timeouts
