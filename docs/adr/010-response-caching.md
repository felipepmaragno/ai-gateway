# ADR-010: Response Caching Strategy

## Status

Accepted

## Context

LLM API calls are:
- **Expensive** — $0.01-0.06 per 1K tokens
- **Slow** — 1-30+ seconds per request
- **Deterministic** — Same input often produces similar output (with temperature=0)

Caching identical requests can significantly reduce costs and latency.

## Decision

Implement **request-based caching** with configurable TTL using Redis.

## Rationale

- **Cost reduction** — Cached responses are free
- **Latency improvement** — Sub-millisecond response for cache hits
- **Scalability** — Reduces load on LLM providers
- **Transparency** — Cache hit indicated in response metadata

## Implementation

### Cache Key Generation

```go
func GenerateCacheKey(req *ChatRequest) string {
    // Normalize request for consistent hashing
    normalized := struct {
        Model       string
        Messages    []Message
        Temperature float64
        MaxTokens   int
    }{
        Model:       req.Model,
        Messages:    req.Messages,
        Temperature: getTemperature(req),
        MaxTokens:   getMaxTokens(req),
    }
    
    data, _ := json.Marshal(normalized)
    hash := sha256.Sum256(data)
    return "cache:" + hex.EncodeToString(hash[:16])
}
```

### Cache Behavior

| Condition | Cached? |
|-----------|---------|
| `temperature = 0` | Yes |
| `temperature > 0` | No (non-deterministic) |
| `stream = true` | No (streaming not cacheable) |
| Identical request | Yes (if within TTL) |

### Response Metadata

```json
{
    "x_gateway": {
        "cache_hit": true,
        "cost_usd": 0,
        "latency_ms": 2
    }
}
```

### TTL Strategy

Default TTL: 1 hour (configurable per request)

```go
cache.Set(ctx, key, response, 1*time.Hour)
```

## Consequences

### Positive
- Dramatic cost savings for repeated queries
- Sub-millisecond latency for cache hits
- Reduced provider rate limit consumption
- Transparent to API consumers

### Negative
- Memory/storage costs for cache
- Stale responses possible (mitigated by TTL)
- Cache key collisions theoretically possible (SHA-256 truncated)

## Cache Invalidation

Currently: TTL-based only

Future options:
- Manual invalidation API
- Model version-based invalidation
- Tenant-specific cache clearing

## Metrics

| Metric | Description |
|--------|-------------|
| `aigateway_cache_hits_total` | Number of cache hits |
| `aigateway_cache_misses_total` | Number of cache misses |

Cache hit ratio = hits / (hits + misses)

## Alternatives Considered

### No Caching
- Simpler but expensive
- Higher latency for all requests

### Client-Side Caching
- Requires client changes
- No cross-client benefit
- Harder to manage TTL

### Semantic Caching
- Cache similar (not identical) requests
- More complex, requires embeddings
- Future enhancement possibility
