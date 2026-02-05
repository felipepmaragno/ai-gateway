# Observability Stack

Prometheus + Grafana for monitoring the AI Gateway.

## Quick Start

```bash
# Start the full stack
docker-compose up -d

# Start the gateway (in another terminal)
go run ./cmd/aigateway

# Access Grafana
open http://localhost:3000
# Login: admin / admin
```

## Components

| Service | Port | Description |
|---------|------|-------------|
| Prometheus | 9090 | Metrics collection and storage |
| Grafana | 3000 | Dashboards and visualization |

## Dashboard Panels

The AI Gateway dashboard includes:

### Overview Row
- **Request Rate** - Requests per second
- **P95 Latency** - 95th percentile response time
- **Total Cost** - Cumulative cost in USD
- **Active Streams** - Current SSE connections

### Performance Row
- **Request Rate by Provider** - Traffic distribution
- **Latency by Provider** - P50/P95/P99 per provider

### Usage Row
- **Token Rate** - Input/output tokens per second
- **Cost per Hour by Tenant** - Spending trends

### Health Row
- **Cache Hit Ratio** - Cache effectiveness
- **Rate Limit Hits** - Throttled requests
- **Circuit Breaker State** - Provider health (Closed/Half-Open/Open)

### Errors Row
- **Provider Errors** - Error rate by provider and type
- **Budget Usage** - Budget consumption per tenant

## Configuration

### Prometheus

Edit `observability/prometheus.yml` to change scrape targets:

```yaml
scrape_configs:
  - job_name: 'aigateway'
    static_configs:
      - targets: ['host.docker.internal:8080']  # Gateway address
```

### Grafana

- Datasources: `observability/grafana/provisioning/datasources/`
- Dashboards: `observability/grafana/dashboards/`

## Adding Alerts

Create alert rules in Prometheus or Grafana:

```yaml
# Example: High error rate alert
groups:
  - name: aigateway
    rules:
      - alert: HighErrorRate
        expr: sum(rate(aigateway_provider_errors_total[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High error rate detected
```

## Useful PromQL Queries

```promql
# Request rate
sum(rate(aigateway_requests_total[5m]))

# Error rate
sum(rate(aigateway_requests_total{status!="success"}[5m])) / sum(rate(aigateway_requests_total[5m]))

# P99 latency
histogram_quantile(0.99, sum(rate(aigateway_request_duration_seconds_bucket[5m])) by (le))

# Cost per hour
sum(increase(aigateway_cost_usd_total[1h]))

# Cache hit ratio
sum(rate(aigateway_cache_hits_total[5m])) / (sum(rate(aigateway_cache_hits_total[5m])) + sum(rate(aigateway_cache_misses_total[5m])))
```
