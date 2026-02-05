# Cost Package

Usage tracking and cost calculation for LLM requests.

## Overview

Tracks token usage per tenant and calculates costs based on model pricing.
Supports budget monitoring with configurable alerts.

## Components

### Calculator

Calculates cost based on model and token usage:

```go
calc := cost.NewCalculator()
costUSD := calc.Calculate("gpt-4", usage)
```

### Pricing

Default pricing per 1K tokens:

| Model | Input | Output |
|-------|-------|--------|
| gpt-4 | $0.03 | $0.06 |
| gpt-4-turbo | $0.01 | $0.03 |
| gpt-4o | $0.005 | $0.015 |
| gpt-4o-mini | $0.00015 | $0.0006 |
| gpt-3.5-turbo | $0.0005 | $0.0015 |
| claude-3-opus | $0.015 | $0.075 |
| claude-3-sonnet | $0.003 | $0.015 |

Custom pricing can be set:
```go
calc.SetPricing("custom-model", cost.ModelPricing{
    InputPer1K:  0.01,
    OutputPer1K: 0.02,
})
```

### Usage Tracker

Records and queries usage per tenant:

```go
type Tracker interface {
    Record(ctx context.Context, record UsageRecord) error
    GetTenantUsage(ctx context.Context, tenantID string, since time.Time) ([]UsageRecord, error)
    GetTenantTotalCost(ctx context.Context, tenantID string, since time.Time) (float64, error)
}
```

### Usage Record

```go
type UsageRecord struct {
    TenantID     string
    RequestID    string
    Model        string
    Provider     string
    InputTokens  int
    OutputTokens int
    CostUSD      float64
    Timestamp    time.Time
}
```

## Backends

| Backend | Use Case | Persistence |
|---------|----------|-------------|
| In-Memory | Development, testing | No |
| PostgreSQL | Production | Yes |

## Performance

Benchmarks:
- Cost calculation: ~9ns/op
- Record (in-memory): ~430ns/op
- GetTenantTotalCost: ~5.5µs/op (1000 records)

## Dependencies

- `internal/domain` - Usage types
- `internal/repository` - PostgreSQL storage (optional)
