# AI Gateway Kubernetes Deployment

This directory contains Kubernetes manifests for deploying the AI Gateway with horizontal scaling support.

## Prerequisites

- Kubernetes 1.25+
- kubectl configured
- Redis deployed in cluster
- PostgreSQL deployed in cluster (optional)
- Metrics Server installed (for HPA)

## Quick Start

```bash
# Build and push image
docker build -t your-registry/ai-gateway:latest .
docker push your-registry/ai-gateway:latest

# Update image in deployment.yaml
# Then apply all manifests
kubectl apply -k k8s/
```

## Manifests

| File | Description |
|------|-------------|
| `namespace.yaml` | Creates `ai-gateway` namespace |
| `serviceaccount.yaml` | Service account for pods |
| `configmap.yaml` | Non-sensitive configuration |
| `secret.yaml` | Sensitive configuration (API keys, DB URLs) |
| `deployment.yaml` | Main deployment with 2 replicas |
| `service.yaml` | ClusterIP service |
| `hpa.yaml` | Horizontal Pod Autoscaler (2-10 replicas) |
| `pdb.yaml` | Pod Disruption Budget (min 1 available) |
| `kustomization.yaml` | Kustomize configuration |

## Configuration

### ConfigMap (configmap.yaml)

```yaml
ADDR: ":8080"
LOG_LEVEL: "info"
DEFAULT_PROVIDER: "ollama"
USE_DISTRIBUTED_CB: "true"    # Enable distributed circuit breaker
SHUTDOWN_TIMEOUT: "30"        # Graceful shutdown timeout
DRAIN_TIMEOUT: "15"           # Connection drain timeout
```

### Secrets (secret.yaml)

**Important:** Update these values before deploying!

```yaml
REDIS_URL: "redis://redis:6379"
DATABASE_URL: "postgres://..."
OPENAI_API_KEY: "sk-..."
ANTHROPIC_API_KEY: "sk-ant-..."
ENCRYPTION_KEY: "your-32-byte-key"
```

## Horizontal Pod Autoscaler

The HPA is configured to:

- **Min replicas:** 2
- **Max replicas:** 10
- **Scale up trigger:** CPU > 70% or Memory > 80%
- **Scale down:** Gradual (10% per minute, 5 min stabilization)
- **Scale up:** Aggressive (up to 4 pods per 15s)

## Health Checks

| Probe | Endpoint | Purpose |
|-------|----------|---------|
| Liveness | `/health/live` | Restart if unresponsive |
| Readiness | `/health/ready` | Remove from LB if deps unhealthy |

## Graceful Shutdown

The deployment is configured for graceful shutdown:

1. `terminationGracePeriodSeconds: 45` - K8s waits 45s before SIGKILL
2. `DRAIN_TIMEOUT: 15` - App waits 15s for connections to drain
3. `SHUTDOWN_TIMEOUT: 30` - App waits 30s for HTTP server shutdown

## Pod Anti-Affinity

Pods prefer to be scheduled on different nodes for high availability:

```yaml
podAntiAffinity:
  preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      topologyKey: kubernetes.io/hostname
```

## Security

- Runs as non-root user (UID 1000)
- Read-only root filesystem
- No privilege escalation
- All capabilities dropped

## Monitoring

Prometheus annotations are included:

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8080"
prometheus.io/path: "/metrics"
```

## Scaling Manually

```bash
# Scale to 5 replicas
kubectl scale deployment ai-gateway -n ai-gateway --replicas=5

# Check HPA status
kubectl get hpa ai-gateway -n ai-gateway
```

## Troubleshooting

```bash
# Check pod status
kubectl get pods -n ai-gateway

# Check logs
kubectl logs -n ai-gateway -l app.kubernetes.io/name=ai-gateway -f

# Check readiness
kubectl exec -n ai-gateway deploy/ai-gateway -- wget -qO- localhost:8080/health/ready

# Check HPA events
kubectl describe hpa ai-gateway -n ai-gateway
```
