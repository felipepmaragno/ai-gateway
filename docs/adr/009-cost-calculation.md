# ADR-009: Cost Calculation Strategy

## Status

Accepted

## Context

LLM APIs charge based on token usage (input + output). Organizations need:
- Visibility into spending per tenant/model/provider
- Budget enforcement to prevent runaway costs
- Cost attribution for chargeback

## Decision

Implement **per-request cost calculation** with configurable pricing and budget monitoring.

## Rationale

- **Transparency** — Every response includes cost in `x_gateway.cost_usd`
- **Control** — Budget limits prevent unexpected bills
- **Flexibility** — Pricing can be updated without code changes
- **Attribution** — Costs tracked per tenant for billing/chargeback

## Implementation

### Pricing Model

```go
type ModelPricing struct {
    InputPer1K  float64 // Cost per 1000 input tokens
    OutputPer1K float64 // Cost per 1000 output tokens
}

var DefaultPricing = map[string]ModelPricing{
    "gpt-4":           {0.03, 0.06},
    "gpt-4-turbo":     {0.01, 0.03},
    "gpt-3.5-turbo":   {0.0005, 0.0015},
    "claude-3-opus":   {0.015, 0.075},
    "claude-3-sonnet": {0.003, 0.015},
    "claude-3-haiku":  {0.00025, 0.00125},
}
```

### Cost Calculation

```go
func (c *Calculator) Calculate(model string, usage Usage) float64 {
    pricing := c.getPricing(model)
    inputCost := float64(usage.PromptTokens) / 1000 * pricing.InputPer1K
    outputCost := float64(usage.CompletionTokens) / 1000 * pricing.OutputPer1K
    return inputCost + outputCost
}
```

### Usage Tracking

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

### Budget Monitoring

```go
type BudgetMonitor struct {
    thresholds []float64 // e.g., [0.5, 0.8, 1.0]
    onAlert    func(Alert)
}

// Check after each request
func (m *BudgetMonitor) Check(tenant *Tenant, currentSpend float64) {
    ratio := currentSpend / tenant.BudgetUSD
    for _, threshold := range m.thresholds {
        if ratio >= threshold {
            m.onAlert(Alert{TenantID: tenant.ID, Threshold: threshold})
        }
    }
}
```

## Response Extension

Cost is included in every response:

```json
{
    "choices": [...],
    "usage": {
        "prompt_tokens": 100,
        "completion_tokens": 50,
        "total_tokens": 150
    },
    "x_gateway": {
        "cost_usd": 0.0045,
        "provider": "openai"
    }
}
```

## Consequences

### Positive
- Real-time cost visibility
- Proactive budget alerts
- Per-tenant cost attribution
- Supports billing/chargeback models

### Negative
- Pricing must be maintained manually
- Cached responses show $0 cost (may confuse users)
- Budget check is post-request (can exceed by one request)

## Limitations

1. **Pricing accuracy** — Hardcoded prices may drift from actual provider pricing
2. **Post-request enforcement** — Budget exceeded after request completes
3. **No streaming cost** — Streaming responses don't include token counts until complete

## Future Improvements

- [ ] Dynamic pricing from provider APIs
- [ ] Pre-request cost estimation for budget enforcement
- [ ] Cost allocation tags per request
- [ ] Spending forecasts and anomaly detection
