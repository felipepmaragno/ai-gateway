# AI Gateway

A centralized proxy for LLM requests with multi-provider support, automatic fallback, cost tracking, and observability.

## Problem

Companies integrating LLMs in production face:
- **Vendor lock-in** — No fallback when provider goes down
- **Uncontrolled costs** — No visibility into spending per service
- **Rate limits** — Requests fail during peak usage
- **Scattered secrets** — API keys in every microservice

## Solution

AI Gateway is an **OpenAI-compatible proxy** that:
- Abstracts multiple providers (OpenAI, Anthropic, Bedrock, Ollama)
- Provides automatic fallback and circuit breaker
- Tracks costs per tenant
- Centralizes observability with OpenTelemetry

```go
// Any service using OpenAI SDK can switch to the gateway
client := openai.NewClient("gw-xxx", openai.WithBaseURL("http://aigateway:8080/v1"))
```

## Features

| Feature | Description |
|---------|-------------|
| **Multi-provider** | OpenAI, Anthropic, AWS Bedrock, Ollama |
| **Automatic fallback** | If provider A fails, tries provider B |
| **Circuit breaker** | Isolates failing providers |
| **Rate limiting** | Per-tenant request quotas (Redis or in-memory) |
| **Cost tracking** | Per-request cost calculation with budget alerts |
| **Response caching** | Cache deterministic responses (Redis or in-memory) |
| **Streaming (SSE)** | Real-time chat responses |
| **OpenTelemetry** | Distributed tracing and Prometheus metrics |
| **Admin API** | Full tenant management CRUD |
| **AWS Integration** | Bedrock, Secrets Manager, SQS, SNS |

## Quick Start

### Prerequisites

- Go 1.22+
- Ollama (for free local testing)
- Redis (optional, for distributed rate limiting/cache)

### Run locally

```bash
# Clone the repository
git clone https://github.com/felipepmaragno/ai-gateway.git
cd ai-gateway

# Start Ollama with a model
ollama pull llama3.2
ollama serve

# Run the gateway (uses in-memory storage by default)
go run ./cmd/aigateway
```

The gateway starts at `http://localhost:8080`.

---

## Testing the Features

### 1. Health Check

```bash
curl -s http://localhost:8080/health | jq
```

Response:
```json
{
  "status": "healthy",
  "version": "0.4.0",
  "providers": { "ollama": "ok" },
  "circuit_breakers": {}
}
```

### 2. List Available Models

```bash
curl -s http://localhost:8080/v1/models \
  -H "Authorization: Bearer gw-default-key" | jq
```

### 3. Chat Completion (Sync)

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer gw-default-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.2",
    "messages": [{"role": "user", "content": "What is 2+2?"}]
  }' | jq
```

Response includes gateway metadata:
```json
{
  "id": "chatcmpl-...",
  "model": "llama3.2",
  "choices": [{"message": {"role": "assistant", "content": "4"}}],
  "usage": {"prompt_tokens": 10, "completion_tokens": 5},
  "x_gateway": {
    "provider": "ollama",
    "latency_ms": 234,
    "cost_usd": 0.00015,
    "cache_hit": false,
    "request_id": "req-abc123",
    "trace_id": "trace-xyz"
  }
}
```

### 4. Chat Completion (Streaming)

```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer gw-default-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.2",
    "messages": [{"role": "user", "content": "Tell me a joke"}],
    "stream": true
  }'
```

### 5. Response Caching

Make the same request twice — the second will be a cache hit:

```bash
# First request (cache miss)
curl -s http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer gw-default-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "llama3.2", "messages": [{"role": "user", "content": "Hi"}], "temperature": 0}' | jq '.x_gateway.cache_hit'
# Output: false

# Second request (cache hit)
curl -s http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer gw-default-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "llama3.2", "messages": [{"role": "user", "content": "Hi"}], "temperature": 0}' | jq '.x_gateway.cache_hit'
# Output: true
```

### 6. Usage & Cost Tracking

```bash
curl -s http://localhost:8080/v1/usage \
  -H "Authorization: Bearer gw-default-key" | jq
```

Response:
```json
{
  "tenant_id": "default",
  "period_start": "2026-02-01T00:00:00Z",
  "total_cost_usd": 0.0023,
  "budget_usd": 1000,
  "budget_used_pct": 0.00023,
  "request_count": 15
}
```

---

## Admin API

Full tenant management without authentication (for internal use).

### List Tenants

```bash
curl -s http://localhost:8080/admin/tenants | jq
```

### Create Tenant

```bash
curl -s -X POST http://localhost:8080/admin/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-service",
    "rate_limit_rpm": 100,
    "budget_usd": 50.0
  }' | jq
```

Response:
```json
{
  "id": "abc123",
  "name": "my-service",
  "api_key": "gw-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "rate_limit_rpm": 100,
  "budget_usd": 50
}
```

### Get Tenant

```bash
curl -s http://localhost:8080/admin/tenants/{id} | jq
```

### Update Tenant

```bash
curl -s -X PUT http://localhost:8080/admin/tenants/{id} \
  -H "Content-Type: application/json" \
  -d '{"rate_limit_rpm": 200, "budget_usd": 100}' | jq
```

### Delete Tenant

```bash
curl -s -X DELETE http://localhost:8080/admin/tenants/{id}
```

### Rotate API Key

```bash
curl -s -X POST http://localhost:8080/admin/tenants/{id}/rotate-key | jq
```

### Admin API Authentication (RBAC)

Enable with `ADMIN_AUTH_ENABLED=true`. Default credentials: `admin:admin`

```bash
# Without auth - returns 401
curl http://localhost:8080/admin/tenants
# Unauthorized

# With Basic Auth
curl -u admin:admin http://localhost:8080/admin/tenants
```

**Roles and Permissions:**

| Role | tenant:read | tenant:write | tenant:delete | usage:read | admin:manage |
|------|:-----------:|:------------:|:-------------:|:----------:|:------------:|
| **admin** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **editor** | ✅ | ✅ | ❌ | ✅ | ❌ |
| **viewer** | ✅ | ❌ | ❌ | ✅ | ❌ |

---

## PostgreSQL Setup

For persistent storage, set `DATABASE_URL` and run migrations:

```bash
# Start PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=aigateway \
  -p 5432:5432 postgres:16

# Run migrations
psql $DATABASE_URL -f migrations/001_initial.up.sql

# Start gateway with PostgreSQL
DATABASE_URL="postgres://postgres:postgres@localhost:5432/aigateway?sslmode=disable" \
  go run ./cmd/aigateway
```

---

## Prometheus Metrics

Available at `http://localhost:8080/metrics`:

| Metric | Description |
|--------|-------------|
| `aigateway_requests_total` | Total requests by tenant, provider, model, status |
| `aigateway_request_duration_seconds` | Request latency histogram |
| `aigateway_tokens_total` | Token usage by type (input/output) |
| `aigateway_cost_usd_total` | Cost in USD by tenant/provider/model |
| `aigateway_active_streams` | Current active streaming connections |
| `aigateway_circuit_breaker_state` | Circuit breaker state (0=closed, 1=open) |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | Server listen address |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `REDIS_URL` | - | Redis URL for distributed cache/rate limiting |
| `OPENAI_API_KEY` | - | OpenAI API key |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI base URL |
| `ANTHROPIC_API_KEY` | - | Anthropic API key |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama server URL |
| `AWS_REGION` | - | AWS region for Bedrock |
| `DEFAULT_PROVIDER` | `ollama` | Default provider when not specified |
| `OTLP_ENDPOINT` | - | OpenTelemetry collector endpoint |
| `ENCRYPTION_KEY` | - | AES-256 key for API key encryption |
| `ADMIN_AUTH_ENABLED` | `false` | Enable Basic Auth for Admin API |
| `USE_DISTRIBUTED_CB` | `false` | Use Redis-backed distributed circuit breaker |
| `SHUTDOWN_TIMEOUT` | `30` | Graceful shutdown timeout (seconds) |
| `DRAIN_TIMEOUT` | `15` | Connection drain timeout (seconds) |

---

## Architecture

```mermaid
flowchart TB
    subgraph Clients
        C1[Service A]
        C2[Service B]
    end

    subgraph AI Gateway
        AUTH[Auth] --> RL[Rate Limiter]
        RL --> CACHE[Cache]
        CACHE --> ROUTER[Router]
        ROUTER --> CB[Circuit Breaker]
    end

    subgraph Providers
        OPENAI[OpenAI]
        ANTHROPIC[Anthropic]
        BEDROCK[Bedrock]
        OLLAMA[Ollama]
    end

    subgraph Storage
        REDIS[(Redis)]
        PG[(PostgreSQL)]
    end

    C1 & C2 --> AUTH
    CB --> OPENAI & ANTHROPIC & BEDROCK & OLLAMA
    RL & CACHE -.-> REDIS
    AUTH -.-> PG
```

### Request Flow

```mermaid
sequenceDiagram
    participant Client
    participant Gateway
    participant Cache
    participant Provider

    Client->>Gateway: POST /v1/chat/completions
    Gateway->>Gateway: Auth + Rate Limit
    Gateway->>Cache: Check Cache
    
    alt Cache Hit
        Cache-->>Client: Cached Response
    else Cache Miss
        Gateway->>Provider: Forward Request
        Provider-->>Gateway: Response
        Gateway->>Cache: Store Response
        Gateway-->>Client: Response + x_gateway
    end
```

### Provider Fallback

```mermaid
flowchart LR
    REQ([Request]) --> P1{Provider 1}
    P1 -->|Success| RESP([Response])
    P1 -->|Fail| P2{Provider 2}
    P2 -->|Success| RESP
    P2 -->|Fail| P3{Provider 3}
    P3 -->|Success| RESP
    P3 -->|Fail| ERR([502 Error])
```

> **📊 See [docs/diagrams.md](docs/diagrams.md) for complete architecture diagrams** including circuit breaker states, rate limiting, cost tracking, RBAC, and horizontal scaling.

---

## Development

```bash
# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Build
go build -o bin/aigateway ./cmd/aigateway

# Run with environment variables
OPENAI_API_KEY=sk-xxx ANTHROPIC_API_KEY=sk-ant-xxx go run ./cmd/aigateway
```

---

## Roadmap

- [x] v0.1.0 — Basic proxy + OpenAI + Ollama + rate limiting
- [x] v0.2.0 — Anthropic + fallback + circuit breaker + cache
- [x] v0.3.0 — Cost tracking + streaming + OpenTelemetry + Prometheus
- [x] v0.4.0 — AWS Bedrock + Secrets Manager + SQS/SNS + Admin API
- [x] v0.5.0 — PostgreSQL persistence + API key encryption + RBAC

## License

MIT
