package circuitbreaker

import (
	"context"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func TestCircuitBreaker_StartsClosedState(t *testing.T) {
	ctx := context.Background()
	cb := New(DefaultConfig())

	if cb.State(ctx) != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State(ctx))
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cfg := Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}
	cb := New(cfg)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		cb.RecordFailure(ctx)
	}

	if cb.State(ctx) != StateOpen {
		t.Errorf("expected StateOpen after %d failures, got %v", cfg.FailureThreshold, cb.State(ctx))
	}
}

func TestCircuitBreaker_BlocksWhenOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}
	cb := New(cfg)
	ctx := context.Background()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	err := cb.Allow(ctx)
	if err != domain.ErrCircuitBreakerOpen {
		t.Errorf("expected ErrCircuitBreakerOpen, got %v", err)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	}
	cb := New(cfg)
	ctx := context.Background()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	time.Sleep(60 * time.Millisecond)

	err := cb.Allow(ctx)
	if err != nil {
		t.Errorf("expected nil after timeout, got %v", err)
	}

	if cb.State(ctx) != StateHalfOpen {
		t.Errorf("expected StateHalfOpen, got %v", cb.State(ctx))
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}
	cb := New(cfg)
	ctx := context.Background()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	time.Sleep(60 * time.Millisecond)
	cb.Allow(ctx)

	cb.RecordSuccess(ctx)
	cb.RecordSuccess(ctx)

	if cb.State(ctx) != StateClosed {
		t.Errorf("expected StateClosed after successes, got %v", cb.State(ctx))
	}
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}
	cb := New(cfg)
	ctx := context.Background()

	cb.RecordFailure(ctx)
	cb.RecordFailure(ctx)

	time.Sleep(60 * time.Millisecond)
	cb.Allow(ctx)

	cb.RecordFailure(ctx)

	if cb.State(ctx) != StateOpen {
		t.Errorf("expected StateOpen after failure in half-open, got %v", cb.State(ctx))
	}
}

func TestManager_GetCreatesBreaker(t *testing.T) {
	m := NewManager(DefaultConfig())

	cb1 := m.Get("provider1")
	cb2 := m.Get("provider1")

	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance for same provider")
	}

	cb3 := m.Get("provider2")
	if cb1 == cb3 {
		t.Error("expected different circuit breaker for different provider")
	}
}
