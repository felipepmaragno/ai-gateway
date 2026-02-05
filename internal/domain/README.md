# Domain Package

Core data types and errors shared across the application.

## Overview

Contains the fundamental types that define the API contract and internal data structures.
This package has no external dependencies to avoid circular imports.

## Types

### Tenant

Represents a customer/organization using the gateway:

```go
type Tenant struct {
    ID                string    // Unique identifier (UUID)
    Name              string    // Display name
    APIKey            string    // Plain API key (only for creation response)
    APIKeyHash        string    // SHA-256 hash for lookup
    BudgetUSD         float64   // Monthly budget limit
    RateLimitRPM      int       // Requests per minute
    AllowedModels     []string  // Whitelisted models (empty = all)
    DefaultProvider   string    // Preferred provider
    FallbackProviders []string  // Fallback order
    Enabled           bool      // Active status
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

### ChatRequest / ChatResponse

OpenAI-compatible chat completion types:

```go
type ChatRequest struct {
    Model       string    // Model identifier (e.g., "gpt-4")
    Messages    []Message // Conversation history
    Temperature *float64  // Randomness (0-2)
    MaxTokens   *int      // Response length limit
    Stream      bool      // Enable streaming
    TopP        *float64  // Nucleus sampling
    Stop        []string  // Stop sequences
}

type ChatResponse struct {
    ID      string   // Response identifier
    Object  string   // "chat.completion"
    Created int64    // Unix timestamp
    Model   string   // Model used
    Choices []Choice // Generated responses
    Usage   Usage    // Token counts
    Gateway *Gateway // Gateway metadata (custom)
}
```

### Gateway Metadata

Custom extension added to responses:

```go
type Gateway struct {
    Provider  string  // Provider that handled the request
    LatencyMs int64   // Total processing time
    CostUSD   float64 // Estimated cost
    CacheHit  bool    // Whether response was cached
    RequestID string  // Unique request identifier
    TraceID   string  // OpenTelemetry trace ID
}
```

### Streaming

For Server-Sent Events (SSE) streaming:

```go
type StreamChunk struct {
    ID      string   // Chunk identifier
    Object  string   // "chat.completion.chunk"
    Created int64    // Unix timestamp
    Model   string   // Model used
    Choices []Choice // Partial response (uses Delta)
}

type Delta struct {
    Role    string // Only in first chunk
    Content string // Incremental content
}
```

## Errors

Sentinel errors for consistent error handling:

| Error | HTTP Status | Description |
|-------|-------------|-------------|
| `ErrTenantNotFound` | 401 | Invalid or missing API key |
| `ErrInvalidAPIKey` | 401 | Malformed API key |
| `ErrRateLimitExceeded` | 429 | Too many requests |
| `ErrBudgetExceeded` | 402 | Monthly budget depleted |
| `ErrProviderNotFound` | 502 | No provider available |
| `ErrProviderError` | 502 | Provider returned error |
| `ErrModelNotAllowed` | 403 | Model not in tenant whitelist |
| `ErrCircuitBreakerOpen` | 503 | Provider temporarily unavailable |
| `ErrInvalidRequest` | 400 | Malformed request body |

## Usage

```go
import "github.com/felipepmaragno/ai-gateway/internal/domain"

// Check for specific errors
if errors.Is(err, domain.ErrRateLimitExceeded) {
    w.WriteHeader(http.StatusTooManyRequests)
}

// Create a request
req := domain.ChatRequest{
    Model: "gpt-4",
    Messages: []domain.Message{
        {Role: "user", Content: "Hello!"},
    },
}
```

## Design Decisions

1. **OpenAI-compatible**: Request/response types match OpenAI API for easy migration
2. **Gateway extension**: Custom `x_gateway` field provides observability without breaking compatibility
3. **Sentinel errors**: Enables `errors.Is()` checks for type-safe error handling
4. **No dependencies**: Avoids circular imports by having no internal package dependencies
