# ADR-002: Ollama as Development Provider

## Status

Accepted

## Context

Testing LLM integrations requires making API calls to providers like OpenAI or Anthropic, which:
- Incur costs per request
- Have rate limits that can slow down development
- Require API keys that may not be available to all developers
- Add latency due to network round-trips

We need a way to develop and test the gateway without these constraints.

## Decision

Include **Ollama** as a first-class provider in the gateway.

## Rationale

- **Free** — No API costs, runs entirely locally
- **OpenAI-compatible** — Ollama provides an OpenAI-compatible API endpoint
- **Fast feedback** — No network latency for local development
- **Self-contained** — Developers can work offline
- **Real LLM responses** — Unlike mocks, provides actual model outputs

## Consequences

### Positive

- Developers can test the full request flow without spending money
- CI/CD pipelines can run integration tests with real LLM responses
- Faster development iteration
- Works offline

### Negative

- Ollama models (Llama 3, Mistral) are less capable than GPT-4/Claude
- Requires local GPU for good performance (or CPU with slower inference)
- Additional setup step for developers

## Implementation

1. Implement `ollama.Adapter` following the same `Provider` interface
2. Set Ollama as default provider in development configuration
3. Document Ollama setup in README
4. Use Ollama in integration tests

## Alternatives Considered

### Mock Server Only

- Faster but doesn't test real LLM behavior
- Good for unit tests, not sufficient for integration tests

### Groq Free Tier

- Good option for CI/CD
- Still requires network, API key management
- Rate limits apply

### Local OpenAI-compatible Server (vLLM, llama.cpp)

- More complex setup than Ollama
- Ollama provides simpler developer experience
