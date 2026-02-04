package cost

import (
	"context"
	"sync"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type ModelPricing struct {
	InputPer1K  float64
	OutputPer1K float64
}

var defaultPricing = map[string]ModelPricing{
	"gpt-4":                      {InputPer1K: 0.03, OutputPer1K: 0.06},
	"gpt-4-turbo":                {InputPer1K: 0.01, OutputPer1K: 0.03},
	"gpt-4o":                     {InputPer1K: 0.005, OutputPer1K: 0.015},
	"gpt-4o-mini":                {InputPer1K: 0.00015, OutputPer1K: 0.0006},
	"gpt-3.5-turbo":              {InputPer1K: 0.0005, OutputPer1K: 0.0015},
	"claude-3-5-sonnet-20241022": {InputPer1K: 0.003, OutputPer1K: 0.015},
	"claude-3-5-haiku-20241022":  {InputPer1K: 0.001, OutputPer1K: 0.005},
	"claude-3-opus-20240229":     {InputPer1K: 0.015, OutputPer1K: 0.075},
	"claude-3-sonnet-20240229":   {InputPer1K: 0.003, OutputPer1K: 0.015},
	"claude-3-haiku-20240307":    {InputPer1K: 0.00025, OutputPer1K: 0.00125},
}

type Calculator struct {
	pricing map[string]ModelPricing
}

func NewCalculator() *Calculator {
	return &Calculator{
		pricing: defaultPricing,
	}
}

func (c *Calculator) Calculate(model string, usage domain.Usage) float64 {
	pricing, ok := c.pricing[model]
	if !ok {
		return 0
	}

	inputCost := float64(usage.PromptTokens) / 1000 * pricing.InputPer1K
	outputCost := float64(usage.CompletionTokens) / 1000 * pricing.OutputPer1K

	return inputCost + outputCost
}

func (c *Calculator) SetPricing(model string, pricing ModelPricing) {
	c.pricing[model] = pricing
}

type UsageRecord struct {
	TenantID     string
	RequestID    string
	Model        string
	Provider     string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Cached       bool
	LatencyMs    int64
	Timestamp    time.Time
}

type Tracker interface {
	Record(ctx context.Context, record UsageRecord) error
	GetTenantUsage(ctx context.Context, tenantID string, since time.Time) ([]UsageRecord, error)
	GetTenantTotalCost(ctx context.Context, tenantID string, since time.Time) (float64, error)
}

type InMemoryTracker struct {
	mu      sync.RWMutex
	records []UsageRecord
}

func NewInMemoryTracker() *InMemoryTracker {
	return &InMemoryTracker{
		records: make([]UsageRecord, 0),
	}
}

func (t *InMemoryTracker) Record(ctx context.Context, record UsageRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.records = append(t.records, record)
	return nil
}

func (t *InMemoryTracker) GetTenantUsage(ctx context.Context, tenantID string, since time.Time) ([]UsageRecord, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []UsageRecord
	for _, r := range t.records {
		if r.TenantID == tenantID && r.Timestamp.After(since) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (t *InMemoryTracker) GetTenantTotalCost(ctx context.Context, tenantID string, since time.Time) (float64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total float64
	for _, r := range t.records {
		if r.TenantID == tenantID && r.Timestamp.After(since) {
			total += r.CostUSD
		}
	}
	return total, nil
}

func (t *InMemoryTracker) GetAllRecords() []UsageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]UsageRecord, len(t.records))
	copy(result, t.records)
	return result
}
