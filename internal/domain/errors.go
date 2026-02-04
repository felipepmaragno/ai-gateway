package domain

import "errors"

var (
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrInvalidAPIKey      = errors.New("invalid API key")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
	ErrProviderNotFound   = errors.New("provider not found")
	ErrProviderError      = errors.New("provider error")
	ErrInvalidRequest     = errors.New("invalid request")
	ErrModelNotAllowed    = errors.New("model not allowed for tenant")
	ErrBudgetExceeded     = errors.New("budget exceeded")
	ErrCircuitBreakerOpen = errors.New("circuit breaker open")
)
