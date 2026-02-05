package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func BenchmarkInMemoryRateLimiter_Allow(b *testing.B) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(ctx, "tenant-1", 10000)
	}
}

func BenchmarkInMemoryRateLimiter_Allow_Parallel(b *testing.B) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow(ctx, "tenant-1", 10000)
		}
	})
}

func BenchmarkInMemoryRateLimiter_MultipleTenants(b *testing.B) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tenantID := fmt.Sprintf("tenant-%d", i%100)
			rl.Allow(ctx, tenantID, 1000)
			i++
		}
	})
}

func BenchmarkInMemoryRateLimiter_HighContention(b *testing.B) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	var wg sync.WaitGroup
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg.Add(10)
		for j := 0; j < 10; j++ {
			go func() {
				defer wg.Done()
				rl.Allow(ctx, "tenant-1", 10000)
			}()
		}
		wg.Wait()
	}
}
