// Package ratelimit provides request rate limiting per tenant.
// It uses a sliding window algorithm to control requests-per-minute (RPM).
// Supports both in-memory (single instance) and Redis (distributed) backends.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// RateLimiter defines the interface for rate limiting backends.
// Returns whether the request is allowed, remaining quota, and reset time.
type RateLimiter interface {
	Allow(ctx context.Context, tenantID string, limit int) (allowed bool, remaining int, resetAt time.Time, err error)
}

// InMemoryRateLimiter implements rate limiting using in-memory sliding windows.
// Suitable for single-instance deployments.
type InMemoryRateLimiter struct {
	mu      sync.Mutex
	windows map[string]*window
}

type window struct {
	count   int
	resetAt time.Time
}

func NewInMemoryRateLimiter() *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		windows: make(map[string]*window),
	}
}

func (r *InMemoryRateLimiter) Allow(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowDuration := time.Minute

	w, ok := r.windows[tenantID]
	if !ok || now.After(w.resetAt) {
		w = &window{
			count:   0,
			resetAt: now.Add(windowDuration),
		}
		r.windows[tenantID] = w
	}

	if w.count >= limit {
		return false, 0, w.resetAt, nil
	}

	w.count++
	remaining := limit - w.count

	return true, remaining, w.resetAt, nil
}
