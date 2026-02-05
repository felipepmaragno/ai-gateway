# Config Package

Application configuration from environment variables.

## Overview

Loads configuration from environment variables with sensible defaults.
All configuration is centralized in a single `Config` struct.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | HTTP server listen address |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `REDIS_URL` | - | Redis connection URL (optional) |
| `DATABASE_URL` | - | PostgreSQL connection URL (optional) |
| `OPENAI_API_KEY` | - | OpenAI API key |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI API base URL |
| `ANTHROPIC_API_KEY` | - | Anthropic API key |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama server URL |
| `DEFAULT_PROVIDER` | `ollama` | Default LLM provider |
| `OTLP_ENDPOINT` | - | OpenTelemetry collector endpoint |
| `AWS_REGION` | - | AWS region for Bedrock, SQS, SNS, Secrets Manager |
| `ENCRYPTION_KEY` | - | Key for API key encryption (AES-256) |
| `ADMIN_AUTH_ENABLED` | `false` | Enable Admin API authentication |

## Usage

```go
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Access configuration
server := &http.Server{
    Addr: cfg.Addr,
}

// Check optional features
if cfg.RedisURL != "" {
    // Use Redis for distributed rate limiting
}

if cfg.DatabaseURL != "" {
    // Use PostgreSQL for persistence
}
```

## Provider Configuration

At least one provider must be configured:

### OpenAI
```bash
export OPENAI_API_KEY=sk-...
export OPENAI_BASE_URL=https://api.openai.com/v1  # optional
```

### Anthropic
```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

### Ollama (Local)
```bash
export OLLAMA_BASE_URL=http://localhost:11434
```

### AWS Bedrock
```bash
export AWS_REGION=us-east-1
# Uses IAM credentials from environment/instance profile
```

## Minimal Configuration

For local development with Ollama:
```bash
export OLLAMA_BASE_URL=http://localhost:11434
export DEFAULT_PROVIDER=ollama
```

## Production Configuration

```bash
# Server
export ADDR=:8080
export LOG_LEVEL=info

# Providers
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export DEFAULT_PROVIDER=openai

# Persistence
export DATABASE_URL=postgres://user:pass@host:5432/aigateway?sslmode=require
export REDIS_URL=redis://host:6379

# Observability
export OTLP_ENDPOINT=otel-collector:4317

# Security
export ENCRYPTION_KEY=your-32-byte-key-here
export ADMIN_AUTH_ENABLED=true

# AWS (optional)
export AWS_REGION=us-east-1
```

## Design Decisions

1. **Environment variables only**: No config files to manage, 12-factor app compliant
2. **Sensible defaults**: Works out of the box with Ollama for local development
3. **Optional features**: Redis, PostgreSQL, telemetry are opt-in
4. **No validation**: Fails fast at runtime if required config is missing
