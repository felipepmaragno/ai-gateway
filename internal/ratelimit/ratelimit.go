package ratelimit

import (
	"context"
	"sync"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, tenantID string, limit int) (allowed bool, remaining int, resetAt time.Time, err error)
}

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
