package circuitbreaker

import (
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func TestCircuitBreaker_StartsClosedState(t *testing.T) {
	cb := New(DefaultConfig())

	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cfg := Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}
	cb := New(cfg)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after %d failures, got %v", cfg.FailureThreshold, cb.State())
	}
}

func TestCircuitBreaker_BlocksWhenOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}
	cb := New(cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	err := cb.Allow()
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

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(60 * time.Millisecond)

	err := cb.Allow()
	if err != nil {
		t.Errorf("expected nil after timeout, got %v", err)
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("expected StateHalfOpen, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}
	cb := New(cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after successes, got %v", cb.State())
	}
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}
	cb := New(cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after failure in half-open, got %v", cb.State())
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
