package circuitbreaker

import (
	"context"
	"sync"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
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

type Config struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
}

func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

type CircuitBreaker struct {
	mu               sync.RWMutex
	state            State
	failures         int
	successes        int
	lastFailure      time.Time
	config           Config
}

func New(cfg Config) *CircuitBreaker {
	return &CircuitBreaker{
		state:  StateClosed,
		config: cfg,
	}
}

func (cb *CircuitBreaker) Allow() error {
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

func (cb *CircuitBreaker) RecordSuccess() {
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

func (cb *CircuitBreaker) RecordFailure() {
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

func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

type Manager struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   Config
}

func NewManager(cfg Config) *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
		config:   cfg,
	}
}

func (m *Manager) Get(providerID string) *CircuitBreaker {
	m.mu.RLock()
	cb, ok := m.breakers[providerID]
	m.mu.RUnlock()

	if ok {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cb, ok := m.breakers[providerID]; ok {
		return cb
	}

	cb = New(m.config)
	m.breakers[providerID] = cb
	return cb
}

func (m *Manager) States() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[string]string)
	for id, cb := range m.breakers {
		states[id] = cb.State().String()
	}
	return states
}

type RedisCircuitBreaker struct {
	providerID string
	config     Config
}

func NewRedisCircuitBreaker(ctx context.Context, providerID string, cfg Config) *RedisCircuitBreaker {
	return &RedisCircuitBreaker{
		providerID: providerID,
		config:     cfg,
	}
}
