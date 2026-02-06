package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryRateLimiter_Allow(t *testing.T) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	allowed, remaining, _, err := rl.Allow(ctx, "tenant1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed to be true")
	}
	if remaining != 2 {
		t.Errorf("expected remaining 2, got %d", remaining)
	}

	rl.Allow(ctx, "tenant1", 3)
	rl.Allow(ctx, "tenant1", 3)

	allowed, remaining, _, err = rl.Allow(ctx, "tenant1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected allowed to be false after limit exceeded")
	}
	if remaining != 0 {
		t.Errorf("expected remaining 0, got %d", remaining)
	}
}

func TestInMemoryRateLimiter_DifferentTenants(t *testing.T) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	rl.Allow(ctx, "tenant1", 1)

	allowed, _, _, _ := rl.Allow(ctx, "tenant1", 1)
	if allowed {
		t.Error("tenant1 should be rate limited")
	}

	allowed, _, _, _ = rl.Allow(ctx, "tenant2", 1)
	if !allowed {
		t.Error("tenant2 should not be rate limited")
	}
}

func TestInMemoryRateLimiter_ResetTime(t *testing.T) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	_, _, resetAt, err := rl.Allow(ctx, "tenant1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reset time should be approximately 1 minute from now
	expectedReset := time.Now().Add(time.Minute)
	diff := resetAt.Sub(expectedReset)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("resetAt should be ~1 minute from now, got diff %v", diff)
	}
}

func TestInMemoryRateLimiter_RemainingCount(t *testing.T) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()
	limit := 5

	for i := 0; i < limit; i++ {
		allowed, remaining, _, _ := rl.Allow(ctx, "tenant1", limit)
		expectedRemaining := limit - i - 1

		if !allowed && i < limit {
			t.Errorf("request %d should be allowed", i)
		}
		if remaining != expectedRemaining {
			t.Errorf("request %d: remaining = %d, want %d", i, remaining, expectedRemaining)
		}
	}

	// Next request should be denied
	allowed, remaining, _, _ := rl.Allow(ctx, "tenant1", limit)
	if allowed {
		t.Error("request after limit should be denied")
	}
	if remaining != 0 {
		t.Errorf("remaining after limit = %d, want 0", remaining)
	}
}

func TestInMemoryRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	done := make(chan bool)
	limit := 100

	// Multiple goroutines trying to get rate limited
	for i := 0; i < 10; i++ {
		go func(tenantID string) {
			for j := 0; j < 20; j++ {
				rl.Allow(ctx, tenantID, limit)
			}
			done <- true
		}("tenant1")
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have used up to 200 requests, but limit is 100
	allowed, _, _, _ := rl.Allow(ctx, "tenant1", limit)
	if allowed {
		t.Error("should be rate limited after concurrent access")
	}
}

func TestInMemoryRateLimiter_HighLimit(t *testing.T) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	// Test with high limit
	for i := 0; i < 1000; i++ {
		allowed, _, _, _ := rl.Allow(ctx, "tenant1", 10000)
		if !allowed {
			t.Errorf("request %d should be allowed with high limit", i)
		}
	}
}

func TestInMemoryRateLimiter_ZeroLimit(t *testing.T) {
	rl := NewInMemoryRateLimiter()
	ctx := context.Background()

	// Zero limit should deny all requests
	allowed, remaining, _, _ := rl.Allow(ctx, "tenant1", 0)
	if allowed {
		t.Error("zero limit should deny all requests")
	}
	if remaining != 0 {
		t.Errorf("remaining with zero limit = %d, want 0", remaining)
	}
}
