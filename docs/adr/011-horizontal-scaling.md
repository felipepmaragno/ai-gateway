# ADR 011: Horizontal Scaling Strategy

**Status:** Proposed  
**Date:** 2026-02-06  
**Authors:** AI Gateway Team

---

## Context

The AI Gateway currently runs as a single instance or with limited horizontal scaling capability. Several components maintain in-memory state that is not shared across instances, causing inconsistent behavior when multiple replicas are deployed.

### Current State Analysis

| Component | Current Implementation | Scaling Issue |
|-----------|----------------------|---------------|
| **Circuit Breaker** | In-memory (`sync.RWMutex`) | State not shared; each instance has independent failure counts |
| **Rate Limiter** | Redis (distributed) ✅ | Already supports horizontal scaling |
| **Cache** | Redis (distributed) ✅ | Already supports horizontal scaling |
| **Session/Streaming** | Stateless ✅ | No sticky sessions required |
| **Cost Tracker** | PostgreSQL ✅ | Already supports horizontal scaling |
| **Budget Monitor** | In-memory alert state | Duplicate alerts across instances |

### Problem Statement

When running N replicas:
1. **Circuit breaker inconsistency**: Provider fails on instance A, but instance B continues sending requests
2. **Budget alerts duplication**: Each instance triggers alerts independently
3. **No load balancing awareness**: Instances don't coordinate request distribution
4. **Graceful shutdown**: Active streams terminated without draining

---

## Decision

Implement distributed state management for all stateful components using Redis as the coordination layer.

### Architecture

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │   (L7/Ingress)  │
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   Instance A    │ │   Instance B    │ │   Instance C    │
│   (Stateless)   │ │   (Stateless)   │ │   (Stateless)   │
└────────┬────────┘ └────────┬────────┘ └────────┬────────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              ▼                             ▼
     ┌─────────────────┐           ┌─────────────────┐
     │     Redis       │           │   PostgreSQL    │
     │  (State Store)  │           │  (Persistence)  │
     │                 │           │                 │
     │ - Circuit State │           │ - Tenants       │
     │ - Rate Limits   │           │ - Usage Records │
     │ - Cache         │           │ - Admin Users   │
     │ - Alert Locks   │           │                 │
     └─────────────────┘           └─────────────────┘
```

---

## Implementation Plan

### Phase 1: Distributed Circuit Breaker (Priority: Critical)

**Problem:** Each instance maintains independent circuit breaker state.

**Solution:** Redis-backed circuit breaker with atomic operations.

```go
// internal/circuitbreaker/redis.go

type RedisCircuitBreaker struct {
    client     *redis.Client
    providerID string
    config     Config
}

// Key structure:
// cb:{provider}:state     -> "closed" | "open" | "half-open"
// cb:{provider}:failures  -> int (failure count)
// cb:{provider}:successes -> int (success count in half-open)
// cb:{provider}:last_fail -> timestamp

func (cb *RedisCircuitBreaker) Allow(ctx context.Context) error {
    // Lua script for atomic state check and transition
    script := `
        local state = redis.call('GET', KEYS[1])
        if state == 'open' then
            local lastFail = tonumber(redis.call('GET', KEYS[2]))
            local timeout = tonumber(ARGV[1])
            if (redis.call('TIME')[1] - lastFail) > timeout then
                redis.call('SET', KEYS[1], 'half-open')
                redis.call('SET', KEYS[3], 0)
                return 'half-open'
            end
            return 'open'
        end
        return state or 'closed'
    `
    // Execute and check result
}

func (cb *RedisCircuitBreaker) RecordFailure(ctx context.Context) {
    // Lua script for atomic increment and state transition
    script := `
        local failures = redis.call('INCR', KEYS[1])
        redis.call('SET', KEYS[2], redis.call('TIME')[1])
        local state = redis.call('GET', KEYS[3])
        
        if state == 'half-open' then
            redis.call('SET', KEYS[3], 'open')
            redis.call('SET', KEYS[4], 0)
            return 'open'
        end
        
        if failures >= tonumber(ARGV[1]) then
            redis.call('SET', KEYS[3], 'open')
            return 'open'
        end
        return state or 'closed'
    `
}
```

**Files to modify:**
- `internal/circuitbreaker/redis.go` - Complete implementation
- `internal/circuitbreaker/circuitbreaker.go` - Add interface
- `internal/router/router.go` - Use distributed circuit breaker
- `cmd/aigateway/main.go` - Initialize Redis circuit breaker

---

### Phase 2: Distributed Budget Alert Deduplication (Priority: High)

**Problem:** Multiple instances trigger the same budget alert.

**Solution:** Redis-based distributed locking for alert deduplication.

```go
// internal/budget/distributed.go

type DistributedMonitor struct {
    *Monitor
    redis    *redis.Client
    lockTTL  time.Duration
}

func (m *DistributedMonitor) Check(ctx context.Context, tenant *domain.Tenant) (AlertLevel, error) {
    level, err := m.Monitor.Check(ctx, tenant)
    if err != nil || level == AlertNone {
        return level, err
    }
    
    // Try to acquire lock for this alert
    lockKey := fmt.Sprintf("budget:alert:%s:%s", tenant.ID, level)
    acquired, err := m.redis.SetNX(ctx, lockKey, "1", m.lockTTL).Result()
    if err != nil || !acquired {
        return AlertNone, nil // Another instance already sent this alert
    }
    
    return level, nil
}
```

**Files to modify:**
- `internal/budget/distributed.go` - New file
- `cmd/aigateway/main.go` - Initialize distributed monitor

---

### Phase 3: Graceful Shutdown with Connection Draining (Priority: Medium)

**Problem:** Active streams terminated immediately on shutdown.

**Solution:** Implement connection draining with configurable timeout.

```go
// cmd/aigateway/main.go

func run() error {
    // ... existing setup ...
    
    // Track active connections
    activeConns := &sync.WaitGroup{}
    
    // Wrap handler to track connections
    trackedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        activeConns.Add(1)
        defer activeConns.Done()
        mux.ServeHTTP(w, r)
    })
    
    srv := &http.Server{
        Handler: trackedHandler,
        // ...
    }
    
    // Graceful shutdown
    go func() {
        <-quit
        slog.Info("initiating graceful shutdown...")
        
        // Stop accepting new connections
        srv.SetKeepAlivesEnabled(false)
        
        // Wait for active connections with timeout
        done := make(chan struct{})
        go func() {
            activeConns.Wait()
            close(done)
        }()
        
        select {
        case <-done:
            slog.Info("all connections drained")
        case <-time.After(cfg.DrainTimeout):
            slog.Warn("drain timeout exceeded, forcing shutdown")
        }
        
        srv.Shutdown(shutdownCtx)
    }()
}
```

**Files to modify:**
- `cmd/aigateway/main.go` - Add connection tracking
- `internal/config/config.go` - Add `DrainTimeout` config

---

### Phase 4: Health Check Improvements (Priority: Medium)

**Problem:** Health checks don't reflect cluster-wide state.

**Solution:** Add readiness probe that checks all dependencies.

```go
// internal/api/handler.go

func (h *Handler) handleHealthReady(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    
    checks := map[string]error{
        "redis":    h.checkRedis(ctx),
        "postgres": h.checkPostgres(ctx),
    }
    
    allHealthy := true
    results := make(map[string]string)
    for name, err := range checks {
        if err != nil {
            results[name] = err.Error()
            allHealthy = false
        } else {
            results[name] = "ok"
        }
    }
    
    status := http.StatusOK
    if !allHealthy {
        status = http.StatusServiceUnavailable
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": allHealthy,
        "checks": results,
    })
}
```

**Files to modify:**
- `internal/api/handler.go` - Enhance readiness probe
- `internal/api/handler.go` - Add dependency health checks

---

### Phase 5: Kubernetes Deployment Configuration (Priority: Medium)

**New files to create:**

```yaml
# deploy/kubernetes/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-gateway
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: ai-gateway
  template:
    metadata:
      labels:
        app: ai-gateway
    spec:
      terminationGracePeriodSeconds: 60
      containers:
      - name: ai-gateway
        image: ai-gateway:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "1000m"
            memory: "512Mi"
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          failureThreshold: 3
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: ai-gateway-secrets
              key: redis-url
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: ai-gateway-secrets
              key: database-url
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ai-gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ai-gateway
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
```

---

## Migration Strategy

### Step 1: Feature Flags
Add feature flags to enable distributed components gradually:

```go
type Config struct {
    // ...existing fields...
    
    // Horizontal scaling features
    UseDistributedCircuitBreaker bool   `env:"USE_DISTRIBUTED_CB" default:"false"`
    UseDistributedBudgetMonitor  bool   `env:"USE_DISTRIBUTED_BUDGET" default:"false"`
    DrainTimeout                 time.Duration `env:"DRAIN_TIMEOUT" default:"30s"`
}
```

### Step 2: Gradual Rollout
1. Deploy with `USE_DISTRIBUTED_CB=false` (current behavior)
2. Enable on staging: `USE_DISTRIBUTED_CB=true`
3. Monitor for 1 week
4. Enable on production with single replica
5. Scale to multiple replicas

### Step 3: Validation
- Verify circuit breaker state is consistent across instances
- Verify budget alerts are not duplicated
- Verify graceful shutdown drains connections
- Load test with multiple replicas

---

## Consequences

### Positive
- **True horizontal scaling**: All instances share state
- **Consistent behavior**: Circuit breaker trips affect all instances
- **No duplicate alerts**: Distributed locking prevents spam
- **Zero-downtime deployments**: Connection draining ensures no dropped requests
- **Auto-scaling ready**: HPA can scale based on load

### Negative
- **Redis dependency**: Redis becomes critical path for circuit breaker
- **Increased latency**: Redis calls add ~1ms per request
- **Operational complexity**: More infrastructure to manage
- **Cost**: Redis cluster for HA adds cost

### Mitigations
- **Redis failure**: Fall back to in-memory circuit breaker
- **Latency**: Use Redis pipelining and connection pooling
- **Complexity**: Provide Helm chart with sensible defaults

---

## Implementation Checklist

### Phase 1: Distributed Circuit Breaker
- [ ] Define `CircuitBreaker` interface
- [ ] Implement `RedisCircuitBreaker`
- [ ] Add Lua scripts for atomic operations
- [ ] Update `Router` to use interface
- [ ] Add feature flag
- [ ] Add integration tests
- [ ] Update metrics to include instance ID

### Phase 2: Distributed Budget Monitor
- [ ] Implement `DistributedMonitor`
- [ ] Add Redis lock for alert deduplication
- [ ] Add feature flag
- [ ] Add integration tests

### Phase 3: Graceful Shutdown
- [ ] Add connection tracking
- [ ] Implement drain timeout
- [ ] Add config option
- [ ] Test with streaming requests

### Phase 4: Health Checks
- [ ] Enhance `/health/ready` with dependency checks
- [ ] Add Redis health check
- [ ] Add PostgreSQL health check
- [ ] Return proper HTTP status codes

### Phase 5: Kubernetes
- [ ] Create Deployment manifest
- [ ] Create HPA manifest
- [ ] Create Service manifest
- [ ] Create ConfigMap/Secret templates
- [ ] Document deployment process

---

## Estimated Effort

| Phase | Effort | Priority |
|-------|--------|----------|
| Phase 1: Distributed Circuit Breaker | 3-4 days | Critical |
| Phase 2: Distributed Budget Monitor | 1-2 days | High |
| Phase 3: Graceful Shutdown | 1 day | Medium |
| Phase 4: Health Checks | 0.5 day | Medium |
| Phase 5: Kubernetes | 1-2 days | Medium |

**Total: 7-10 days**

---

## References

- [Redis Distributed Locks (Redlock)](https://redis.io/docs/manual/patterns/distributed-locks/)
- [Kubernetes Graceful Shutdown](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination)
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
