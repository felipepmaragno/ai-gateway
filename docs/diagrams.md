# AI Gateway - Architecture Diagrams

This document contains Mermaid diagrams illustrating the main flows and architecture of the AI Gateway.

---

## 1. High-Level Architecture

```mermaid
flowchart TB
    subgraph Clients
        C1[Service A]
        C2[Service B]
        C3[Service C]
    end

    subgraph AI Gateway
        LB[Load Balancer]
        
        subgraph Instance["Gateway Instance"]
            AUTH[Auth Middleware]
            RL[Rate Limiter]
            CACHE[Response Cache]
            ROUTER[Provider Router]
            CB[Circuit Breaker]
            COST[Cost Calculator]
        end
    end

    subgraph Providers
        OPENAI[OpenAI API]
        ANTHROPIC[Anthropic API]
        BEDROCK[AWS Bedrock]
        OLLAMA[Ollama Local]
    end

    subgraph Storage
        REDIS[(Redis)]
        PG[(PostgreSQL)]
    end

    subgraph Observability
        PROM[Prometheus]
        OTEL[OpenTelemetry]
        GRAFANA[Grafana]
    end

    C1 & C2 & C3 --> LB
    LB --> AUTH
    AUTH --> RL
    RL --> CACHE
    CACHE --> ROUTER
    ROUTER --> CB
    CB --> OPENAI & ANTHROPIC & BEDROCK & OLLAMA

    RL -.-> REDIS
    CACHE -.-> REDIS
    AUTH -.-> PG
    COST -.-> PG

    Instance -.-> PROM
    Instance -.-> OTEL
    PROM --> GRAFANA
```

---

## 2. Request Flow - Chat Completion

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Gateway
    participant Auth
    participant RateLimiter
    participant Cache
    participant Router
    participant Provider
    participant CostTracker

    Client->>Gateway: POST /v1/chat/completions
    Gateway->>Auth: Validate API Key
    
    alt Invalid API Key
        Auth-->>Client: 401 Unauthorized
    end

    Auth->>Gateway: Tenant Info
    Gateway->>RateLimiter: Check Rate Limit
    
    alt Rate Limit Exceeded
        RateLimiter-->>Client: 429 Too Many Requests
    end

    RateLimiter->>Gateway: Allowed (remaining: N)
    Gateway->>Cache: Check Cache (key=hash(request))
    
    alt Cache Hit
        Cache-->>Client: 200 OK (cached response)
    end

    Cache->>Router: Select Provider
    Router->>Router: Check Circuit Breakers
    Router->>Provider: Forward Request
    
    alt Provider Error
        Provider-->>Router: Error
        Router->>Router: Try Fallback Provider
    end

    Provider-->>Router: Response
    Router-->>Gateway: Response
    Gateway->>Cache: Store Response
    Gateway->>CostTracker: Record Usage
    Gateway-->>Client: 200 OK (with x_gateway metadata)
```

---

## 3. Provider Fallback with Circuit Breaker

```mermaid
flowchart TD
    START([Request]) --> SELECT[Select Primary Provider]
    SELECT --> CB_CHECK{Circuit Breaker<br/>Open?}
    
    CB_CHECK -->|Closed| TRY_PRIMARY[Try Primary Provider]
    CB_CHECK -->|Open| FALLBACK[Select Fallback Provider]
    
    TRY_PRIMARY --> PRIMARY_OK{Success?}
    PRIMARY_OK -->|Yes| RECORD_SUCCESS[Record Success]
    PRIMARY_OK -->|No| RECORD_FAILURE[Record Failure]
    
    RECORD_SUCCESS --> RESPONSE([Return Response])
    
    RECORD_FAILURE --> THRESHOLD{Failure<br/>Threshold<br/>Reached?}
    THRESHOLD -->|Yes| OPEN_CB[Open Circuit Breaker]
    THRESHOLD -->|No| FALLBACK
    
    OPEN_CB --> FALLBACK
    
    FALLBACK --> FB_AVAILABLE{Fallback<br/>Available?}
    FB_AVAILABLE -->|Yes| TRY_FALLBACK[Try Fallback Provider]
    FB_AVAILABLE -->|No| ERROR([502 Bad Gateway])
    
    TRY_FALLBACK --> FB_OK{Success?}
    FB_OK -->|Yes| RESPONSE
    FB_OK -->|No| FB_AVAILABLE

    subgraph Circuit Breaker States
        direction LR
        CLOSED[Closed] -->|Failures >= Threshold| OPEN[Open]
        OPEN -->|Timeout Elapsed| HALF[Half-Open]
        HALF -->|Success| CLOSED
        HALF -->|Failure| OPEN
    end
```

---

## 4. Rate Limiting Flow

```mermaid
flowchart TD
    REQ([Incoming Request]) --> EXTRACT[Extract Tenant ID]
    EXTRACT --> CHECK[Check Rate Limit]
    
    subgraph Sliding Window Algorithm
        CHECK --> GET_WINDOW[Get Current Window]
        GET_WINDOW --> COUNT[Count Requests in Window]
        COUNT --> COMPARE{Count < Limit?}
    end
    
    COMPARE -->|Yes| ALLOW[Allow Request]
    COMPARE -->|No| DENY[Deny Request]
    
    ALLOW --> INCREMENT[Increment Counter]
    INCREMENT --> SET_HEADERS[Set Rate Limit Headers]
    SET_HEADERS --> PROCESS([Process Request])
    
    DENY --> HEADERS_429[Set Rate Limit Headers]
    HEADERS_429 --> RETURN_429([429 Too Many Requests])
    
    subgraph Response Headers
        H1["X-RateLimit-Limit: 100"]
        H2["X-RateLimit-Remaining: 45"]
        H3["X-RateLimit-Reset: 2026-02-06T15:00:00Z"]
    end
```

---

## 5. Cost Tracking & Budget Monitoring

```mermaid
flowchart TD
    RESPONSE([Provider Response]) --> EXTRACT[Extract Token Usage]
    EXTRACT --> CALC[Calculate Cost]
    
    subgraph Cost Calculation
        CALC --> MODEL[Get Model Pricing]
        MODEL --> INPUT[Input Tokens × Input Price]
        MODEL --> OUTPUT[Output Tokens × Output Price]
        INPUT --> TOTAL[Total Cost = Input + Output]
        OUTPUT --> TOTAL
    end
    
    TOTAL --> RECORD[Record Usage]
    RECORD --> DB[(PostgreSQL)]
    
    RECORD --> CHECK_BUDGET[Check Budget]
    
    subgraph Budget Monitor
        CHECK_BUDGET --> GET_TOTAL[Get Total Spend]
        GET_TOTAL --> PERCENT[Calculate % Used]
        PERCENT --> LEVEL{Alert Level?}
        
        LEVEL -->|>= 90%| CRITICAL[Critical Alert]
        LEVEL -->|>= 75%| WARNING[Warning Alert]
        LEVEL -->|>= 100%| EXCEEDED[Budget Exceeded]
        LEVEL -->|< 75%| OK[No Alert]
    end
    
    CRITICAL --> NOTIFY[Send Notification]
    WARNING --> NOTIFY
    EXCEEDED --> BLOCK[Block Future Requests]
    EXCEEDED --> NOTIFY
```

---

## 6. Authentication & RBAC Flow

```mermaid
flowchart TD
    REQ([Admin API Request]) --> HAS_AUTH{Has Authorization<br/>Header?}
    
    HAS_AUTH -->|No| UNAUTH([401 Unauthorized])
    HAS_AUTH -->|Yes| PARSE[Parse Basic Auth]
    
    PARSE --> LOOKUP[Lookup User]
    LOOKUP --> EXISTS{User Exists?}
    
    EXISTS -->|No| UNAUTH
    EXISTS -->|Yes| VERIFY[Verify Password]
    
    VERIFY --> VALID{Password Valid?}
    VALID -->|No| UNAUTH
    VALID -->|Yes| GET_ROLE[Get User Role]
    
    GET_ROLE --> CHECK_PERM{Has Required<br/>Permission?}
    
    CHECK_PERM -->|No| FORBIDDEN([403 Forbidden])
    CHECK_PERM -->|Yes| PROCESS([Process Request])
    
    subgraph Roles & Permissions
        direction LR
        ADMIN[Admin] --> ALL[All Permissions]
        EDITOR[Editor] --> READ_WRITE[Read + Write]
        VIEWER[Viewer] --> READ_ONLY[Read Only]
    end
```

---

## 7. Streaming Response Flow (SSE)

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Gateway
    participant Provider

    Client->>Gateway: POST /v1/chat/completions<br/>{stream: true}
    Gateway->>Gateway: Validate & Rate Limit
    Gateway->>Provider: Forward Request (stream)
    
    Gateway-->>Client: HTTP 200<br/>Content-Type: text/event-stream

    loop For each chunk
        Provider-->>Gateway: Chunk N
        Gateway-->>Client: data: {chunk N}
    end

    Provider-->>Gateway: [DONE]
    Gateway->>Gateway: Calculate Cost
    Gateway-->>Client: data: {x_gateway: {...}}
    Gateway-->>Client: data: [DONE]
```

---

## 8. Cache Key Generation

```mermaid
flowchart LR
    subgraph Request Fields
        MODEL[model]
        MESSAGES[messages]
        TEMP[temperature]
        MAX[max_tokens]
    end

    MODEL & MESSAGES & TEMP & MAX --> JSON[JSON Serialize]
    JSON --> SHA256[SHA-256 Hash]
    SHA256 --> KEY["cache:{hash}"]
    
    KEY --> REDIS[(Redis/Memory)]
```

---

## 9. Component Dependencies

```mermaid
graph TD
    subgraph External
        REDIS[(Redis)]
        PG[(PostgreSQL)]
        OPENAI[OpenAI]
        ANTHROPIC[Anthropic]
        BEDROCK[Bedrock]
        OLLAMA[Ollama]
    end

    subgraph Core
        HANDLER[API Handler]
        ROUTER[Router]
        AUTH[Auth]
    end

    subgraph Infrastructure
        CACHE[Cache]
        RATELIMIT[Rate Limiter]
        CIRCUIT[Circuit Breaker]
        COST[Cost Tracker]
        BUDGET[Budget Monitor]
        METRICS[Metrics]
        TELEMETRY[Telemetry]
    end

    HANDLER --> AUTH
    HANDLER --> RATELIMIT
    HANDLER --> CACHE
    HANDLER --> ROUTER
    HANDLER --> COST
    HANDLER --> BUDGET
    HANDLER --> METRICS
    HANDLER --> TELEMETRY

    ROUTER --> CIRCUIT
    ROUTER --> OPENAI & ANTHROPIC & BEDROCK & OLLAMA

    RATELIMIT -.->|optional| REDIS
    CACHE -.->|optional| REDIS
    AUTH -.->|optional| PG
    COST -.->|optional| PG

    classDef optional stroke-dasharray: 5 5
    class REDIS,PG optional
```

---

## 10. Horizontal Scaling Architecture (Planned)

```mermaid
flowchart TB
    subgraph Internet
        CLIENTS[Clients]
    end

    subgraph Kubernetes Cluster
        LB[Load Balancer / Ingress]
        
        subgraph Pods
            POD1[Gateway Pod 1]
            POD2[Gateway Pod 2]
            POD3[Gateway Pod 3]
        end
        
        HPA[Horizontal Pod Autoscaler]
    end

    subgraph Shared State
        REDIS[(Redis Cluster)]
        PG[(PostgreSQL)]
    end

    subgraph Providers
        OPENAI[OpenAI]
        ANTHROPIC[Anthropic]
    end

    CLIENTS --> LB
    LB --> POD1 & POD2 & POD3
    HPA -.-> Pods

    POD1 & POD2 & POD3 --> REDIS
    POD1 & POD2 & POD3 --> PG
    POD1 & POD2 & POD3 --> OPENAI & ANTHROPIC

    subgraph Shared via Redis
        CB[Circuit Breaker State]
        RL[Rate Limit Counters]
        CACHE[Response Cache]
        ALERTS[Alert Locks]
    end

    REDIS --- CB & RL & CACHE & ALERTS
```

---

## 11. Graceful Shutdown Flow

```mermaid
sequenceDiagram
    autonumber
    participant K8s as Kubernetes
    participant Pod as Gateway Pod
    participant LB as Load Balancer
    participant Redis
    participant Clients

    K8s->>Pod: SIGTERM
    Pod->>Pod: shuttingDown = true
    Pod->>LB: Remove from endpoints
    
    Note over Pod: New requests get 503
    
    Pod->>Pod: SetKeepAlivesEnabled(false)
    
    rect rgb(255, 240, 200)
        Note over Pod,Clients: Connection Draining (up to DRAIN_TIMEOUT)
        Clients->>Pod: In-flight request
        Pod->>Pod: activeConns.Wait()
        Pod-->>Clients: Complete response
    end
    
    alt All connections drained
        Pod->>Pod: "all connections drained"
    else Drain timeout exceeded
        Pod->>Pod: "drain timeout, forcing shutdown"
    end
    
    Pod->>Pod: srv.Shutdown()
    Pod->>Redis: Close connections
    Pod->>K8s: Exit 0
```

---

## 12. Distributed Circuit Breaker State Machine

```mermaid
stateDiagram-v2
    [*] --> Closed
    
    Closed --> Open: failures >= threshold
    Closed --> Closed: success (reset failures)
    
    Open --> HalfOpen: timeout elapsed
    Open --> Open: request blocked
    
    HalfOpen --> Closed: successes >= threshold
    HalfOpen --> Open: any failure
    
    note right of Closed
        All requests allowed
        Failures counted
    end note
    
    note right of Open
        All requests blocked
        Returns ErrCircuitBreakerOpen
    end note
    
    note right of HalfOpen
        Limited requests allowed
        Testing if service recovered
    end note
```

---

## 13. Health Check Decision Flow

```mermaid
flowchart TD
    REQ([GET /health/ready]) --> CHECKERS{Health Checkers<br/>Configured?}
    
    CHECKERS -->|No| OK_SIMPLE([200 OK])
    CHECKERS -->|Yes| RUN[Run All Checks<br/>Concurrently]
    
    RUN --> REDIS[Redis Ping]
    RUN --> PG[Postgres Ping]
    
    REDIS --> COLLECT[Collect Results]
    PG --> COLLECT
    
    COLLECT --> ALL_OK{All Healthy?}
    
    ALL_OK -->|Yes| READY([200 Ready])
    ALL_OK -->|No| NOT_READY([503 Not Ready])
    
    subgraph Response
        READY --> R1["status: ready<br/>checks: {redis: ok, postgres: ok}"]
        NOT_READY --> R2["status: not_ready<br/>checks: {redis: error, ...}"]
    end
```

---

## Viewing These Diagrams

These diagrams use [Mermaid](https://mermaid.js.org/) syntax and can be rendered:

1. **GitHub** - Renders automatically in markdown files
2. **VS Code** - Install "Markdown Preview Mermaid Support" extension
3. **Online** - Use [Mermaid Live Editor](https://mermaid.live/)
4. **Documentation** - Tools like Docusaurus, MkDocs support Mermaid
