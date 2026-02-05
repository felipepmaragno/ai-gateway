# Rate Limit Package

Request rate limiting per tenant using token bucket algorithm.

## Overview

Controls request rates to prevent abuse and ensure fair usage across tenants.
Each tenant has a configurable requests-per-minute (RPM) limit.

## Backends

| Backend | Use Case | Distributed |
|---------|----------|-------------|
| In-Memory | Single instance | No |
| Redis | Multiple instances | Yes |

## Algorithm

Uses **token bucket** algorithm:
- Bucket fills at rate of `limit/minute` tokens
- Each request consumes 1 token
- Request denied if bucket is empty
- Bucket capacity equals the limit

## Interface

```go
type RateLimiter interface {
    Allow(ctx context.Context, tenantID string, limit int) (bool, error)
}
```

## Usage

```go
limiter := ratelimit.NewInMemoryRateLimiter()

// Check if request is allowed (60 RPM limit)
allowed, err := limiter.Allow(ctx, tenant.ID, 60)
if !allowed {
    // Return 429 Too Many Requests
}
```

## Redis Backend

For distributed deployments, use Redis backend:

```go
limiter := ratelimit.NewRedisRateLimiter(redisClient)
```

Redis implementation uses Lua scripts for atomic operations.

## Performance

Benchmarks (in-memory):
- Single tenant: ~67ns/op
- Parallel access: ~134ns/op
- Multi-tenant: ~272ns/op

## Configuration

Rate limits are configured per-tenant in the database:
- `rate_limit_rpm`: Requests per minute allowed

## HTTP Response

When rate limited, the API returns:
```json
{
  "error": {
    "message": "rate limit exceeded",
    "type": "rate_limit_error"
  }
}
```

With headers:
- `Retry-After: 60` (seconds until reset)

## Dependencies

- `github.com/redis/go-redis/v9` - Redis client (optional)
