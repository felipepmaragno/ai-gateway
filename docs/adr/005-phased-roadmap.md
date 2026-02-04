# ADR-005: Phased Roadmap

## Status

Accepted

## Context

The initial specification included many features for v0.1.0:
- Multi-provider (OpenAI, Anthropic, Bedrock)
- Fallback + Circuit breaker
- Rate limiting + Cost tracking
- Cache + Streaming
- AWS integrations
- OpenTelemetry

This scope is too large for an MVP and risks:
- Never shipping a working version
- Losing focus on core functionality
- Difficulty tracking progress

## Decision

Split the roadmap into **four phases**, each delivering incremental value.

## Phases

### v0.1.0 — Foundation

Focus: **Basic proxy that works**

- OpenAI-compatible API (POST /v1/chat/completions)
- Provider: OpenAI
- Provider: Ollama (for testing)
- Rate limiting per tenant
- Basic Prometheus metrics
- Health checks
- Structured logging (slog)

**Why this scope:**
- Delivers working software quickly
- Enables testing without costs (Ollama)
- Establishes foundation for future features

### v0.2.0 — Resilience

Focus: **Handle failures gracefully**

- Provider: Anthropic
- Automatic fallback between providers
- Circuit breaker per provider
- Response cache (Redis)

**Why this scope:**
- Handles real-world failure scenarios
- Enables provider flexibility
- Cache reduces costs and latency

### v0.3.0 — Observability & Streaming

Focus: **Production visibility**

- Cost tracking per request/tenant
- Budget alerts
- Streaming (SSE) support
- OpenTelemetry tracing
- Enhanced metrics

**Why this scope:**
- Production systems need visibility
- Streaming is essential for chat UX
- Cost tracking enables budget management

### v0.4.0 — AWS Integration

Focus: **Cloud-native features**

- Provider: AWS Bedrock
- AWS Secrets Manager integration
- SQS async mode
- SNS notifications
- Complete admin API

**Why this scope:**
- AWS is widely used in production
- Enables enterprise deployment patterns
- Completes the feature set

## Consequences

### Positive

- Clear milestones for progress tracking
- Each version is shippable
- Can stop at any phase with working software
- Easier to estimate and plan

### Negative

- Some features delayed to later versions
- May need refactoring as features are added
- Users of early versions have limited features

## Success Criteria

Each phase is complete when:
1. All listed features are implemented
2. Tests pass (unit + integration)
3. Documentation is updated
4. README reflects current capabilities
5. Can be demonstrated end-to-end
