# Cache Package

Response caching for deterministic LLM requests.

## Overview

Caches LLM responses to reduce latency and costs for identical requests.
Only caches when `temperature=0` or not set (deterministic outputs).

## Backends

| Backend | Use Case | Persistence |
|---------|----------|-------------|
| In-Memory | Single instance, development | No |
| Redis | Distributed, production | Yes |

## Key Generation

Cache keys are SHA-256 hashes of:
- Model name
- Messages array
- Temperature
- Max tokens

```go
key := cache.GenerateCacheKey(req)
// Returns: "cache:a1b2c3d4..."
```

## Interface

```go
type Cache interface {
    Get(ctx context.Context, key string) (*domain.ChatResponse, bool)
    Set(ctx context.Context, key string, resp *domain.ChatResponse, ttl time.Duration) error
}
```

## Usage

```go
// Check cache first
key := cache.GenerateCacheKey(req)
if resp, ok := cache.Get(ctx, key); ok {
    return resp // Cache hit
}

// Call provider
resp, err := provider.ChatCompletion(ctx, req)

// Store in cache (only for deterministic requests)
if req.Temperature == nil || *req.Temperature == 0 {
    cache.Set(ctx, key, resp, 5*time.Minute)
}
```

## TTL Strategy

- Default: 5 minutes
- Configurable per-request via handler config
- Expired entries are cleaned up periodically (in-memory)
- Redis handles expiration natively

## Performance

Benchmarks (in-memory):
- Get (hit): ~69ns
- Get (miss): ~17ns
- Set: ~121ns
- Key generation: ~1µs

## Dependencies

- `internal/domain` - Response types
- `github.com/redis/go-redis/v9` - Redis client (optional)
