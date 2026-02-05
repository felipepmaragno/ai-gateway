package cost

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func BenchmarkInMemoryTracker_Record(b *testing.B) {
	tracker := NewInMemoryTracker()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record := UsageRecord{
			TenantID:     "tenant-1",
			RequestID:    fmt.Sprintf("req-%d", i),
			Model:        "gpt-4",
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.01,
			Timestamp:    time.Now(),
		}
		tracker.Record(ctx, record)
	}
}

func BenchmarkInMemoryTracker_Record_Parallel(b *testing.B) {
	tracker := NewInMemoryTracker()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			record := UsageRecord{
				TenantID:     fmt.Sprintf("tenant-%d", i%10),
				RequestID:    fmt.Sprintf("req-%d", i),
				Model:        "gpt-4",
				Provider:     "openai",
				InputTokens:  100,
				OutputTokens: 50,
				CostUSD:      0.01,
				Timestamp:    time.Now(),
			}
			tracker.Record(ctx, record)
			i++
		}
	})
}

func BenchmarkInMemoryTracker_GetTenantUsage(b *testing.B) {
	tracker := NewInMemoryTracker()
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		record := UsageRecord{
			TenantID:     "tenant-1",
			RequestID:    fmt.Sprintf("req-%d", i),
			Model:        "gpt-4",
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.01,
			Timestamp:    time.Now(),
		}
		tracker.Record(ctx, record)
	}

	since := time.Now().Add(-1 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.GetTenantUsage(ctx, "tenant-1", since)
	}
}

func BenchmarkInMemoryTracker_GetTenantTotalCost(b *testing.B) {
	tracker := NewInMemoryTracker()
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		record := UsageRecord{
			TenantID:     "tenant-1",
			RequestID:    fmt.Sprintf("req-%d", i),
			Model:        "gpt-4",
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.01,
			Timestamp:    time.Now(),
		}
		tracker.Record(ctx, record)
	}

	since := time.Now().Add(-1 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.GetTenantTotalCost(ctx, "tenant-1", since)
	}
}

func BenchmarkCostCalculator_Calculate(b *testing.B) {
	calc := NewCalculator()
	usage := domain.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Calculate("gpt-4", usage)
	}
}
