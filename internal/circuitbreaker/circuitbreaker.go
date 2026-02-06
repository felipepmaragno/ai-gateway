// Package circuitbreaker implements the circuit breaker pattern for failure protection.
// It prevents cascading failures by failing fast when a service is unhealthy.
//
// States:
//   - Closed: Normal operation, requests pass through
//   - Open: Service unhealthy, requests fail immediately
//   - Half-Open: Testing recovery, limited requests allowed
//
// Implementations:
//   - InMemoryCircuitBreaker: Single-instance, uses sync.RWMutex
//   - RedisCircuitBreaker: Distributed, uses Redis with Lua scripts for atomicity
package circuitbreaker

import (
	"context"
	"sync"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

// CircuitBreaker defines the interface for circuit breaker implementations.
// Both in-memory and distributed (Redis) implementations satisfy this interface.
type CircuitBreaker interface {
	// Allow checks if a request should be allowed through.
	// Returns nil if allowed, ErrCircuitBreakerOpen if the circuit is open.
	Allow(ctx context.Context) error

	// RecordSuccess records a successful request.
	// In half-open state, enough successes will close the circuit.
	RecordSuccess(ctx context.Context)

	// RecordFailure records a failed request.
	// Enough failures will open the circuit.
	RecordFailure(ctx context.Context)

	// State returns the current state of the circuit breaker.
	State(ctx context.Context) State
}

// State represents the current state of a circuit breaker.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing fast
	StateHalfOpen              // Testing recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config defines circuit breaker behavior.
type Config struct {
	FailureThreshold int           // Failures before opening
	SuccessThreshold int           // Successes to close from half-open
	Timeout          time.Duration // Time before transitioning to half-open
}

// DefaultConfig returns sensible defaults for most use cases.
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// InMemoryCircuitBreaker tracks failures and controls request flow to a service.
// This implementation is suitable for single-instance deployments.
type InMemoryCircuitBreaker struct {
	mu          sync.RWMutex
	state       State
	failures    int
	successes   int
	lastFailure time.Time
	config      Config
}

// NewInMemory creates a new in-memory circuit breaker.
func NewInMemory(cfg Config) *InMemoryCircuitBreaker {
	return &InMemoryCircuitBreaker{
		state:  StateClosed,
		config: cfg,
	}
}

// New creates a new in-memory circuit breaker (alias for NewInMemory for backward compatibility).
func New(cfg Config) *InMemoryCircuitBreaker {
	return NewInMemory(cfg)
}

func (cb *InMemoryCircuitBreaker) Allow(ctx context.Context) error {
	cb.mu.RLock()
	state := cb.state
	lastFailure := cb.lastFailure
	cb.mu.RUnlock()

	switch state {
	case StateClosed:
		return nil
	case StateOpen:
		if time.Since(lastFailure) > cb.config.Timeout {
			cb.mu.Lock()
			if cb.state == StateOpen {
				cb.state = StateHalfOpen
				cb.successes = 0
			}
			cb.mu.Unlock()
			return nil
		}
		return domain.ErrCircuitBreakerOpen
	case StateHalfOpen:
		return nil
	}

	return nil
}

func (cb *InMemoryCircuitBreaker) RecordSuccess(ctx context.Context) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

func (cb *InMemoryCircuitBreaker) RecordFailure(ctx context.Context) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failures++
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.successes = 0
	}
}

func (cb *InMemoryCircuitBreaker) State(ctx context.Context) State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *InMemoryCircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Manager manages circuit breakers for multiple providers.
// It supports both in-memory and distributed (Redis) backends.
type Manager struct {
	mu       sync.RWMutex
	breakers map[string]CircuitBreaker
	config   Config
	factory  func(providerID string) CircuitBreaker
}

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithRedis configures the manager to use Redis-backed circuit breakers.
func WithRedis(redisURL string) ManagerOption {
	return func(m *Manager) {
		m.factory = func(providerID string) CircuitBreaker {
			cb, err := NewRedis(redisURL, providerID, m.config)
			if err != nil {
				// Fallback to in-memory if Redis fails
				return NewInMemory(m.config)
			}
			return cb
		}
	}
}

// NewManager creates a new circuit breaker manager.
// By default, it uses in-memory circuit breakers.
// Use WithRedis option for distributed circuit breakers.
func NewManager(cfg Config, opts ...ManagerOption) *Manager {
	m := &Manager{
		breakers: make(map[string]CircuitBreaker),
		config:   cfg,
		factory: func(providerID string) CircuitBreaker {
			return NewInMemory(cfg)
		},
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Get returns the circuit breaker for a provider, creating one if it doesn't exist.
func (m *Manager) Get(providerID string) CircuitBreaker {
	m.mu.RLock()
	cb, ok := m.breakers[providerID]
	m.mu.RUnlock()

	if ok {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existingCB, ok := m.breakers[providerID]; ok {
		return existingCB
	}

	cb = m.factory(providerID)
	m.breakers[providerID] = cb
	return cb
}

// States returns the current state of all circuit breakers.
func (m *Manager) States() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctx := context.Background()
	states := make(map[string]string)
	for id, cb := range m.breakers {
		states[id] = cb.State(ctx).String()
	}
	return states
}
