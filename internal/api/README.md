# API Package

HTTP handlers for the AI Gateway.

## Overview

This package implements the HTTP API layer, handling:
- Chat completions (`POST /v1/chat/completions`)
- Model listing (`GET /v1/models`)
- Health checks (`GET /health`)
- Usage reporting (`GET /v1/usage`)

## Architecture

```
Request → Authentication → Rate Limiting → Cache Check → Provider Router → Response
                                              ↓
                                         Cache Store
                                              ↓
                                      Cost Tracking + Metrics
```

## Key Components

### Handler

The main `Handler` struct orchestrates all middleware and business logic:

```go
handler := api.NewHandler(api.HandlerConfig{
    TenantRepo:    tenantRepo,    // Tenant authentication
    RateLimiter:   rateLimiter,   // Request throttling
    Router:        providerRouter, // LLM provider selection
    Cache:         responseCache,  // Response caching
    CostTracker:   costTracker,   // Usage tracking
    BudgetMonitor: budgetMonitor, // Budget alerts
})
```

### Request Flow

1. **Authentication**: Validates API key via `X-API-Key` header
2. **Rate Limiting**: Checks tenant's RPM limit
3. **Cache**: Returns cached response if available (deterministic requests only)
4. **Provider Selection**: Routes to appropriate LLM provider with fallback
5. **Cost Tracking**: Records token usage and costs
6. **Metrics**: Emits Prometheus metrics and OpenTelemetry spans

## Streaming

Streaming responses use Server-Sent Events (SSE):
- Sets `Content-Type: text/event-stream`
- Flushes chunks as they arrive from the provider
- Handles client disconnection gracefully

## Error Handling

All errors return JSON with consistent format:
```json
{
  "error": {
    "message": "error description",
    "type": "error_type"
  }
}
```

## Dependencies

- `internal/domain` - Request/response types
- `internal/router` - Provider selection
- `internal/cache` - Response caching
- `internal/ratelimit` - Rate limiting
- `internal/cost` - Usage tracking
- `internal/metrics` - Prometheus metrics
- `internal/telemetry` - OpenTelemetry tracing
