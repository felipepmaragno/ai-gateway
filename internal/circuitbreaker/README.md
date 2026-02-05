# Circuit Breaker Package

Failure protection for external service calls.

## Overview

Implements the circuit breaker pattern to prevent cascading failures
when LLM providers become unhealthy or unresponsive.

## States

```
     ┌─────────┐
     │  Closed │ ←── Normal operation
     └────┬────┘
          │ failures >= threshold
          ▼
     ┌─────────┐
     │  Open   │ ←── Requests fail fast
     └────┬────┘
          │ timeout elapsed
          ▼
   ┌───────────────┐
   │  Half-Open    │ ←── Test with single request
   └───────┬───────┘
           │
     success → Closed
     failure → Open
```

## Configuration

```go
cb := circuitbreaker.New(circuitbreaker.Config{
    FailureThreshold: 5,           // Failures before opening
    SuccessThreshold: 2,           // Successes to close from half-open
    Timeout:          30 * time.Second, // Time before half-open
})
```

## Interface

```go
type CircuitBreaker interface {
    Execute(fn func() error) error
    State() State
    Reset()
}
```

## Usage

```go
cb := circuitbreaker.New(config)

err := cb.Execute(func() error {
    return provider.HealthCheck(ctx)
})

if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
    // Circuit is open, skip this provider
}
```

## Manager

Manages circuit breakers per provider:

```go
manager := circuitbreaker.NewManager(config)

cb := manager.Get("openai")
err := cb.Execute(func() error {
    return openaiProvider.ChatCompletion(ctx, req)
})
```

## Metrics

The circuit breaker emits metrics:
- `circuit_breaker_state` - Current state (0=closed, 1=open, 2=half-open)
- `circuit_breaker_failures_total` - Total failure count
- `circuit_breaker_successes_total` - Total success count

## Dependencies

None (standalone package)
