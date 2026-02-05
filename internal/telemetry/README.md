# Telemetry Package

Distributed tracing with OpenTelemetry.

## Overview

Provides observability through distributed tracing, allowing you to track requests
across services and identify performance bottlenecks.

## Configuration

Set the OTLP endpoint to enable tracing:
```
OTLP_ENDPOINT=localhost:4317
```

If not set, tracing is disabled but the API remains functional (no-op tracer).

## Initialization

```go
shutdown, err := telemetry.Init(ctx, "ai-gateway", cfg.OTLPEndpoint)
if err != nil {
    log.Fatal(err)
}
defer shutdown(ctx)
```

## Creating Spans

```go
ctx, span := telemetry.StartSpan(ctx, "chat.completions")
defer span.End()

// Add attributes
telemetry.AddRequestAttributes(span, tenantID, provider, model, requestID)
telemetry.AddTokenAttributes(span, inputTokens, outputTokens)
telemetry.AddCostAttribute(span, costUSD)
telemetry.AddCacheAttribute(span, cacheHit)

// Record errors
if err != nil {
    telemetry.AddErrorAttribute(span, err)
}
```

## Span Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| `tenant.id` | string | Tenant identifier |
| `provider` | string | LLM provider used |
| `model` | string | Model name |
| `request.id` | string | Unique request ID |
| `tokens.input` | int | Input token count |
| `tokens.output` | int | Output token count |
| `tokens.total` | int | Total tokens |
| `cost.usd` | float | Request cost in USD |
| `cache.hit` | bool | Whether response was cached |
| `error.message` | string | Error description (if any) |

## Trace ID

Get the trace ID for logging correlation:
```go
traceID := telemetry.GetTraceID(ctx)
slog.Info("request completed", "trace_id", traceID)
```

## Backends

Compatible with any OTLP-compliant backend:
- Jaeger
- Zipkin
- Grafana Tempo
- AWS X-Ray (with collector)
- Datadog

## Span Hierarchy

```
chat.completions
├── rate_limit.check
├── cache.lookup
├── provider.select
├── provider.chat_completion
│   └── http.request
├── cost.calculate
└── cache.store
```

## Dependencies

- `go.opentelemetry.io/otel` - OpenTelemetry SDK
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` - OTLP gRPC exporter
