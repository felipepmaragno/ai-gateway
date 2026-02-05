# Metrics Package

Prometheus metrics for monitoring and alerting.

## Overview

Exposes application metrics in Prometheus format at `/metrics` endpoint.
All metrics are prefixed with `aigateway_`.

## Available Metrics

### Request Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aigateway_requests_total` | Counter | tenant_id, provider, model, status | Total requests processed |
| `aigateway_request_duration_seconds` | Histogram | tenant_id, provider, model | Request latency distribution |

### Token Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aigateway_tokens_total` | Counter | tenant_id, provider, model, type | Total tokens (input/output) |
| `aigateway_cost_usd_total` | Counter | tenant_id, provider, model | Cumulative cost in USD |

### Cache Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aigateway_cache_hits_total` | Counter | tenant_id | Cache hit count |
| `aigateway_cache_misses_total` | Counter | tenant_id | Cache miss count |

### Rate Limiting

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aigateway_rate_limit_hits_total` | Counter | tenant_id | Rate limit rejections |

### Provider Health

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aigateway_circuit_breaker_state` | Gauge | provider | 0=closed, 1=half-open, 2=open |
| `aigateway_provider_errors_total` | Counter | provider, error_type | Provider error count |

### Streaming

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aigateway_active_streams` | Gauge | - | Current active SSE connections |

### Budget

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aigateway_budget_usage_ratio` | Gauge | tenant_id | Budget usage (0.0 to 1.0) |

## Usage

Metrics are recorded automatically by the handler. For custom recording:

```go
// Record a request
metrics.RecordRequest(tenantID, provider, model, "success", 1.5)

// Record tokens
metrics.RecordTokens(tenantID, provider, model, 100, 50)

// Record cost
metrics.RecordCost(tenantID, provider, model, 0.015)

// Record cache
metrics.RecordCacheHit(tenantID)
metrics.RecordCacheMiss(tenantID)

// Record rate limit
metrics.RecordRateLimitHit(tenantID)

// Record provider state
metrics.SetCircuitBreakerState("openai", 0) // closed
metrics.RecordProviderError("openai", "timeout")

// Record budget
metrics.SetBudgetUsage(tenantID, 0.75) // 75% used
```

## Histogram Buckets

Request duration uses these buckets (in seconds):
```
0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120
```

## Example Queries (PromQL)

```promql
# Request rate per tenant
rate(aigateway_requests_total[5m])

# P99 latency
histogram_quantile(0.99, rate(aigateway_request_duration_seconds_bucket[5m]))

# Cache hit ratio
sum(rate(aigateway_cache_hits_total[5m])) / 
(sum(rate(aigateway_cache_hits_total[5m])) + sum(rate(aigateway_cache_misses_total[5m])))

# Cost per hour by tenant
sum by (tenant_id) (increase(aigateway_cost_usd_total[1h]))

# Error rate by provider
sum by (provider) (rate(aigateway_provider_errors_total[5m]))
```

## Grafana Dashboard

Import the dashboard from `dashboards/aigateway.json` (if available) or create panels using the queries above.

## Dependencies

- `github.com/prometheus/client_golang` - Prometheus client library
