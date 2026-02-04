package ratelimit

import (
	"context"
	"testing"
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
