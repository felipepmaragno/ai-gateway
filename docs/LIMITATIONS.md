# Limitations & Future Improvements

This document outlines the current limitations of the AI Gateway and potential improvements for production readiness.

---

## Current Limitations

### 1. Authentication & Security

| Limitation | Impact | Severity |
|------------|--------|----------|
| **No API key rotation** | Keys cannot be rotated without downtime | High |
| **No OAuth/OIDC support** | Limited to API key authentication | Medium |
| **Admin API unprotected by default** | Anyone can create/delete tenants | High |
| **No request signing** | Requests can be replayed | Medium |
| **Encryption key in env var** | Key management is manual | Medium |

### 2. Scalability

| Limitation | Impact | Severity | Status |
|------------|--------|----------|--------|
| **Single-region deployment** | No geo-distribution or failover | High | Planned |
| **No horizontal pod autoscaling config** | Manual scaling required | Medium | [ADR-011](adr/011-horizontal-scaling.md) |
| **In-memory circuit breaker state** | State not shared across instances | High | [ADR-011](adr/011-horizontal-scaling.md) |
| **No connection pooling tuning** | Default pool sizes may be insufficient | Low | — |

> **See [ADR-011: Horizontal Scaling Strategy](adr/011-horizontal-scaling.md)** for the complete implementation plan.

### 3. Observability

| Limitation | Impact | Severity |
|------------|--------|----------|
| **No alerting rules configured** | Manual monitoring required | High |
| **No distributed tracing in prod** | OTLP endpoint optional | Medium |
| **No log aggregation setup** | Logs only in stdout | Medium |
| **No SLO/SLA dashboards** | No availability tracking | Medium |

### 4. Provider Integration

| Limitation | Impact | Severity |
|------------|--------|----------|
| **No streaming for Bedrock** | Only sync requests supported | Medium |
| **No function calling support** | Tool use not implemented | Medium |
| **No vision/multimodal support** | Text-only requests | Low |
| **No embeddings endpoint** | Only chat completions | Low |
| **Fixed retry logic** | No exponential backoff configuration | Low |

### 5. Cost Management

| Limitation | Impact | Severity |
|------------|--------|----------|
| **No real-time budget enforcement** | Budget checked post-request | Medium |
| **Pricing hardcoded** | No dynamic pricing updates | Medium |
| **No cost allocation tags** | Cannot attribute costs to projects | Low |
| **No spending forecasts** | No predictive analytics | Low |

### 6. Data & Storage

| Limitation | Impact | Severity |
|------------|--------|----------|
| **No request/response logging** | Cannot audit or replay requests | Medium |
| **No data retention policies** | Usage data grows unbounded | Medium |
| **No backup/restore procedures** | Manual database management | High |
| **No multi-tenancy isolation** | Shared database schema | Low |

### 7. Operations

| Limitation | Impact | Severity |
|------------|--------|----------|
| **No Kubernetes manifests** | Only docker-compose provided | Medium |
| **No Terraform/IaC** | Manual infrastructure setup | Medium |
| **No CI/CD for deployments** | Only lint/test in CI | Medium |
| **No canary/blue-green deployments** | All-or-nothing updates | Low |

---

## Recommended Improvements

### Phase 1: Production Hardening (1-2 weeks)

#### Security
- [ ] Implement API key rotation with grace period
- [ ] Add JWT/OAuth2 authentication option
- [ ] Enable Admin API authentication by default
- [ ] Add request rate limiting per IP (not just per tenant)
- [ ] Implement audit logging for all admin operations

#### Reliability
- [ ] Add Redis-backed circuit breaker state
- [ ] Configure Kubernetes HPA based on CPU/memory
- [ ] Add health check endpoints for all dependencies
- [ ] Implement graceful degradation when Redis is unavailable

#### Observability
- [ ] Create Prometheus alerting rules (high error rate, latency, budget)
- [ ] Add Grafana alert notifications (Slack, PagerDuty)
- [ ] Configure log shipping to centralized system (Loki, ELK)
- [ ] Add SLO dashboard (availability, latency percentiles)

### Phase 2: Feature Completeness (2-4 weeks)

#### Provider Features
- [ ] Add streaming support for AWS Bedrock
- [ ] Implement function calling / tool use
- [ ] Add embeddings endpoint (`/v1/embeddings`)
- [ ] Support vision models (image inputs)
- [ ] Add Google Vertex AI provider

#### Cost Management
- [ ] Real-time budget enforcement (pre-request check with estimation)
- [ ] Dynamic pricing from provider APIs
- [ ] Cost allocation tags per request
- [ ] Spending forecasts and anomaly detection
- [ ] Budget alerts via SNS/webhook

#### Data Management
- [ ] Request/response logging (opt-in, with PII redaction)
- [ ] Data retention policies with automatic cleanup
- [ ] Database backup automation
- [ ] Read replicas for analytics queries

### Phase 3: Enterprise Features (1-2 months)

#### Multi-tenancy
- [ ] Tenant isolation with separate schemas or databases
- [ ] Custom domains per tenant
- [ ] Tenant-specific rate limits and quotas
- [ ] Self-service tenant portal

#### Advanced Routing
- [ ] A/B testing for model versions
- [ ] Canary deployments for new providers
- [ ] Geographic routing (latency-based)
- [ ] Custom routing rules (model → provider mapping)

#### Compliance
- [ ] SOC 2 compliance documentation
- [ ] GDPR data handling (right to deletion, export)
- [ ] Request encryption at rest
- [ ] Audit trail with tamper-proof logging

#### Infrastructure
- [ ] Kubernetes Helm chart
- [ ] Terraform modules for AWS/GCP/Azure
- [ ] Multi-region deployment with global load balancing
- [ ] Disaster recovery runbooks

---

## Technical Debt

### Code Quality
- [ ] Increase test coverage to >80%
- [ ] Add integration tests for all providers
- [ ] Implement contract testing for provider APIs
- [ ] Add fuzzing for input validation

### Performance
- [ ] Benchmark and optimize hot paths
- [ ] Implement connection pooling tuning
- [ ] Add response compression (gzip)
- [ ] Profile memory allocations

### Documentation
- [ ] API reference with OpenAPI spec
- [x] Architecture decision records (ADRs) — 11 ADRs documented
- [ ] Runbooks for common operations
- [ ] Incident response playbooks

---

## Known Issues

1. **Redis rate limiter precision**: The sliding window implementation may allow slightly more requests than configured during high concurrency.

2. **Cache key collisions**: Cache keys are based on request hash; different requests with same hash will return cached response.

3. **Tenant creation defaults to disabled**: New tenants must be explicitly enabled via PUT request.

4. **No graceful shutdown for streaming**: Active streams are terminated immediately on shutdown.

5. **Circuit breaker not integrated**: Circuit breaker metrics are exported but not enforced in request flow.

---

## Contributing

When addressing these limitations:

1. Create an issue describing the limitation and proposed solution
2. Reference this document in the issue
3. Update this document when the limitation is resolved
4. Add tests for any new functionality
