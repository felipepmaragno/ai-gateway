# ADR-006: Observability Stack

## Status

Accepted

## Context

Production systems require visibility into:
- Request rates and latencies
- Error rates and types
- Resource utilization
- Business metrics (cost, token usage)

We need to decide on the observability stack and what metrics to expose.

## Decision

Use **Prometheus + Grafana** for metrics collection and visualization, with optional **OpenTelemetry** for distributed tracing.

## Rationale

### Prometheus
- **Pull-based model** — Gateway exposes `/metrics`, Prometheus scrapes
- **Industry standard** — Widely adopted, extensive ecosystem
- **PromQL** — Powerful query language for alerting and dashboards
- **Low overhead** — Minimal impact on request latency

### Grafana
- **Rich visualization** — Time series, gauges, heatmaps
- **Alerting** — Built-in alert rules and notifications
- **Provisioning** — Dashboards as code (JSON)
- **Free** — Open source, no licensing costs

### OpenTelemetry (Optional)
- **Distributed tracing** — End-to-end request visibility
- **Vendor-neutral** — Export to Jaeger, Zipkin, or cloud providers
- **Context propagation** — Trace ID in responses for debugging

## Metrics Exposed

### Request Metrics
| Metric | Type | Labels |
|--------|------|--------|
| `aigateway_requests_total` | Counter | tenant_id, provider, model, status |
| `aigateway_request_duration_seconds` | Histogram | tenant_id, provider, model |

### Token Metrics
| Metric | Type | Labels |
|--------|------|--------|
| `aigateway_tokens_total` | Counter | tenant_id, provider, model, type |
| `aigateway_cost_usd_total` | Counter | tenant_id, provider, model |

### Resilience Metrics
| Metric | Type | Labels |
|--------|------|--------|
| `aigateway_circuit_breaker_state` | Gauge | provider |
| `aigateway_rate_limit_hits_total` | Counter | tenant_id |
| `aigateway_cache_hits_total` | Counter | tenant_id |
| `aigateway_cache_misses_total` | Counter | tenant_id |

### Operational Metrics
| Metric | Type | Labels |
|--------|------|--------|
| `aigateway_active_streams` | Gauge | - |
| `aigateway_provider_errors_total` | Counter | provider, error_type |
| `aigateway_budget_usage_ratio` | Gauge | tenant_id |

## Consequences

### Positive
- Full visibility into gateway behavior
- Enables data-driven capacity planning
- Supports SLO/SLA monitoring
- Pre-built dashboard reduces setup time

### Negative
- Additional infrastructure (Prometheus, Grafana)
- Metrics cardinality can grow with tenants/models
- Dashboard maintenance overhead

## Implementation

1. Use `prometheus/client_golang` for metrics
2. Expose `/metrics` endpoint on main server
3. Provide `docker-compose.yml` with Prometheus + Grafana
4. Include pre-configured Grafana dashboard
5. Initialize all gauge metrics at startup (avoid "No data")

## Alternatives Considered

### CloudWatch/Datadog/New Relic
- Vendor lock-in
- Costs money
- Good option for production, not for local development

### StatsD
- Push-based model
- Less powerful querying
- Additional aggregation layer needed
