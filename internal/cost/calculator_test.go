package cost

import (
	"context"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func TestCalculator_Calculate(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		model    string
		usage    domain.Usage
		expected float64
	}{
		{
			name:  "gpt-4 with tokens",
			model: "gpt-4",
			usage: domain.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expected: 0.03 + 0.03, // 1K * 0.03 + 0.5K * 0.06
		},
		{
			name:  "unknown model returns zero",
			model: "unknown-model",
			usage: domain.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expected: 0,
		},
		{
			name:  "gpt-3.5-turbo",
			model: "gpt-3.5-turbo",
			usage: domain.Usage{
				PromptTokens:     2000,
				CompletionTokens: 1000,
			},
			expected: 0.001 + 0.0015, // 2K * 0.0005 + 1K * 0.0015
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Calculate(tt.model, tt.usage)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestInMemoryTracker_Record(t *testing.T) {
	tracker := NewInMemoryTracker()
	ctx := context.Background()

	record := UsageRecord{
		TenantID:     "tenant1",
		RequestID:    "req1",
		Model:        "gpt-4",
		Provider:     "openai",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.01,
		Timestamp:    time.Now(),
	}

	err := tracker.Record(ctx, record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records := tracker.GetAllRecords()
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestInMemoryTracker_GetTenantTotalCost(t *testing.T) {
	tracker := NewInMemoryTracker()
	ctx := context.Background()

	now := time.Now()

	tracker.Record(ctx, UsageRecord{
		TenantID:  "tenant1",
		CostUSD:   0.10,
		Timestamp: now,
	})
	tracker.Record(ctx, UsageRecord{
		TenantID:  "tenant1",
		CostUSD:   0.20,
		Timestamp: now,
	})
	tracker.Record(ctx, UsageRecord{
		TenantID:  "tenant2",
		CostUSD:   0.50,
		Timestamp: now,
	})

	total, err := tracker.GetTenantTotalCost(ctx, "tenant1", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if total < 0.29 || total > 0.31 {
		t.Errorf("expected ~0.30, got %f", total)
	}
}
