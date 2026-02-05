# ADR-007: HTTP Client Timeouts

## Status

Accepted

## Context

LLM API calls can take significant time (10-60+ seconds for long responses). Without proper timeout configuration:
- Requests may hang indefinitely
- Resources (goroutines, connections) may leak
- User experience degrades with no feedback

Go's default `http.Client` has **no timeout**, which is dangerous for production.

## Decision

Create a centralized `httputil` package with **granular timeout configuration** for all HTTP clients.

## Rationale

Different phases of an HTTP request have different failure modes:
- **Dial timeout** — DNS resolution + TCP handshake
- **TLS handshake timeout** — Certificate exchange
- **Response header timeout** — Time to first byte
- **Idle connection timeout** — Keep-alive management
- **Total timeout** — Overall request deadline

Configuring each independently allows fine-tuned behavior.

## Implementation

```go
type ClientConfig struct {
    Timeout               time.Duration // Total request timeout (120s)
    DialTimeout           time.Duration // TCP connection (10s)
    TLSHandshakeTimeout   time.Duration // TLS negotiation (10s)
    ResponseHeaderTimeout time.Duration // Time to first byte (30s)
    IdleConnTimeout       time.Duration // Keep-alive (90s)
    MaxIdleConns          int           // Connection pool size (100)
    MaxIdleConnsPerHost   int           // Per-host pool (10)
}
```

### Default Values

| Setting | Value | Rationale |
|---------|-------|-----------|
| Total Timeout | 120s | LLM responses can be slow |
| Dial Timeout | 10s | Fail fast on network issues |
| TLS Handshake | 10s | Detect certificate problems |
| Response Header | 30s | Provider should respond within 30s |
| Idle Connection | 90s | Balance reuse vs resource usage |

### Usage

All providers use the shared client:

```go
// In provider initialization
client := httputil.DefaultClient()

// Or with custom config
client := httputil.NewClient(httputil.ClientConfig{
    Timeout: 180 * time.Second, // Longer for streaming
})
```

## Consequences

### Positive
- Consistent timeout behavior across all providers
- Prevents resource leaks from hanging connections
- Enables connection pooling for performance
- Single place to tune HTTP behavior

### Negative
- May need per-provider tuning for edge cases
- Streaming requests may need longer timeouts

## Alternatives Considered

### Per-Provider Configuration
- More flexible but harder to maintain
- Inconsistent behavior across providers

### Context-Based Timeouts Only
- Doesn't handle connection-level issues
- Less granular control
