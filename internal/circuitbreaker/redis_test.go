package circuitbreaker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func getRedisURL(t *testing.T) string {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set, skipping Redis circuit breaker tests")
	}
	return url
}

func TestRedisCircuitBreaker_StartsClosedState(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	cb, err := NewRedis(redisURL, "test-provider-1", DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create redis circuit breaker: %v", err)
	}
	defer cb.Reset(ctx)
	defer cb.Close()

	if cb.State(ctx) != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State(ctx))
	}
}

func TestRedisCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	cfg := Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}
	cb, err := NewRedis(redisURL, "test-provider-2", cfg)
	if err != nil {
		t.Fatalf("failed to create redis circuit breaker: %v", err)
	}
	defer cb.Reset(ctx)
	defer cb.Close()

	for i := 0; i < 3; i++ {
		cb.RecordFailure(ctx)
	}

	if cb.State(ctx) != StateOpen {
		t.Errorf("expected StateOpen after %d failures, got %v", cfg.FailureThreshold, cb.State(ctx))
	}
}

func TestRedisCircuitBreaker_BlocksWhenOpen(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}
	cb, err := NewRedis(redisURL, "test-provider-3", cfg)
	if err != nil {
		t.Fatalf("failed to create redis circuit breaker: %v", err)
	}
	defer cb.Reset(ctx)
	defer cb.Close()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	err = cb.Allow(ctx)
	if err != domain.ErrCircuitBreakerOpen {
		t.Errorf("expected ErrCircuitBreakerOpen, got %v", err)
	}
}

func TestRedisCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}
	cb, err := NewRedis(redisURL, "test-provider-4", cfg)
	if err != nil {
		t.Fatalf("failed to create redis circuit breaker: %v", err)
	}
	defer cb.Reset(ctx)
	defer cb.Close()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	// Wait for timeout to elapse
	time.Sleep(1100 * time.Millisecond)

	err = cb.Allow(ctx)
	if err != nil {
		t.Errorf("expected nil after timeout, got %v", err)
	}

	if cb.State(ctx) != StateHalfOpen {
		t.Errorf("expected StateHalfOpen, got %v", cb.State(ctx))
	}
}

func TestRedisCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}
	cb, err := NewRedis(redisURL, "test-provider-5", cfg)
	if err != nil {
		t.Fatalf("failed to create redis circuit breaker: %v", err)
	}
	defer cb.Reset(ctx)
	defer cb.Close()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	// Wait for timeout
	time.Sleep(1100 * time.Millisecond)
	cb.Allow(ctx) // Transition to half-open

	cb.RecordSuccess(ctx)
	cb.RecordSuccess(ctx)

	if cb.State(ctx) != StateClosed {
		t.Errorf("expected StateClosed after successes, got %v", cb.State(ctx))
	}
}

func TestRedisCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}
	cb, err := NewRedis(redisURL, "test-provider-6", cfg)
	if err != nil {
		t.Fatalf("failed to create redis circuit breaker: %v", err)
	}
	defer cb.Reset(ctx)
	defer cb.Close()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	// Wait for timeout
	time.Sleep(1100 * time.Millisecond)
	cb.Allow(ctx) // Transition to half-open

	cb.RecordFailure(ctx)

	if cb.State(ctx) != StateOpen {
		t.Errorf("expected StateOpen after failure in half-open, got %v", cb.State(ctx))
	}
}

func TestRedisCircuitBreaker_Reset(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          30 * time.Second,
	}
	cb, err := NewRedis(redisURL, "test-provider-7", cfg)
	if err != nil {
		t.Fatalf("failed to create redis circuit breaker: %v", err)
	}
	defer cb.Close()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	if cb.State(ctx) != StateOpen {
		t.Errorf("expected StateOpen, got %v", cb.State(ctx))
	}

	err = cb.Reset(ctx)
	if err != nil {
		t.Errorf("reset failed: %v", err)
	}

	if cb.State(ctx) != StateClosed {
		t.Errorf("expected StateClosed after reset, got %v", cb.State(ctx))
	}
}

func TestManager_WithRedisOption(t *testing.T) {
	redisURL := getRedisURL(t)

	m := NewManager(DefaultConfig(), WithRedis(redisURL))

	cb1 := m.Get("redis-provider-1")
	cb2 := m.Get("redis-provider-1")

	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance for same provider")
	}

	// Verify it's actually a Redis circuit breaker by checking type
	if _, ok := cb1.(*RedisCircuitBreaker); !ok {
		t.Error("expected RedisCircuitBreaker type")
	}
}
