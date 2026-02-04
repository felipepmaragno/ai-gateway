# ADR-003: Redis for Distributed State

## Status

Accepted

## Context

The gateway needs to maintain state for:
- **Rate limiting** — Track request counts per tenant
- **Circuit breaker** — Track failure counts per provider
- **Response cache** — Store cached LLM responses

This state must be shared across multiple gateway instances for horizontal scaling.

## Decision

Use **Redis** for all distributed state management.

## Rationale

- **Sub-millisecond latency** — Critical for rate limiting on every request
- **Atomic operations** — INCR, ZADD, etc. for race-free counting
- **TTL support** — Automatic expiration for rate limit windows and cache
- **Battle-tested** — Widely used for these exact use cases
- **Proven in dispatch project** — Already demonstrated competence with Redis

## Consequences

### Positive

- Enables horizontal scaling with shared state
- High performance for hot-path operations
- Rich data structures (sorted sets for sliding window, hashes for circuit breaker)
- Ecosystem of monitoring tools

### Negative

- Additional infrastructure dependency
- Requires Redis availability for gateway to function
- Memory-bound (costs scale with cache size)

## Implementation Details

### Rate Limiting

Use sorted sets (ZSET) for sliding window algorithm:
- Key: `ratelimit:{tenant_id}`
- Score: Unix timestamp
- Value: Request ID
- TTL: Window size (e.g., 60s)

### Circuit Breaker

Use hashes for state:
- Key: `cb:{provider_id}`
- Fields: `state`, `failures`, `successes`, `last_failure_at`

### Response Cache

Use strings with JSON serialization:
- Key: `cache:{request_hash}`
- Value: JSON-encoded response
- TTL: Configurable per request

## Alternatives Considered

### In-Memory Only

- Doesn't support horizontal scaling
- State lost on restart

### PostgreSQL

- Higher latency for hot-path operations
- Overkill for ephemeral state

### Memcached

- Simpler but lacks data structures needed for rate limiting
- No persistence options
