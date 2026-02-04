# ADR-001: OpenAI-Compatible API

## Status

Accepted

## Context

We need to define the API contract for the AI Gateway. Options considered:

1. **Custom API** — Design our own request/response format
2. **OpenAI-compatible API** — Mirror OpenAI's API structure
3. **GraphQL** — Flexible query language

## Decision

Implement an **OpenAI-compatible API** as the primary interface.

## Rationale

- **Zero migration cost** — Services already using OpenAI SDK only need to change the base URL
- **Ecosystem compatibility** — Works with existing tools, libraries, and documentation
- **Industry standard** — Most LLM providers (Anthropic, Groq, Together) offer OpenAI-compatible endpoints
- **Reduced learning curve** — Developers familiar with OpenAI API can use the gateway immediately

## Consequences

### Positive

- Drop-in replacement for existing OpenAI integrations
- Can leverage existing OpenAI client libraries
- Familiar API for most developers

### Negative

- Limited to features that fit OpenAI's API model
- Must maintain compatibility as OpenAI evolves their API
- Some provider-specific features may be harder to expose

## Alternatives Rejected

### Custom API

- Would require all consumers to learn a new API
- Would need custom client libraries
- No ecosystem benefits

### GraphQL

- Overkill for request/response pattern
- Adds complexity without clear benefit
- Not standard in LLM space
