# ADR-004: Optional AWS Integration

## Status

Accepted

## Context

AWS services provide powerful capabilities for production deployments. However, requiring AWS for local development creates friction:
- Developers need AWS accounts and credentials
- Costs money even for development/testing
- Adds complexity to getting started

## Decision

Make AWS services (Secrets Manager, SQS, SNS) **optional** with interface-based abstraction.

## Rationale

- **Simplifies local development** — Core functionality works with just PostgreSQL and Redis
- **Demonstrates good design** — Interface-based architecture enables flexibility
- **Enables testing** — Can use LocalStack or in-memory implementations
- **Gradual adoption** — Teams can start without AWS and add it later

## Consequences

### Positive

- Lower barrier to entry for contributors
- Faster local development setup
- Clean separation of concerns
- Easy to test without cloud dependencies

### Negative

- Additional abstraction layer
- Must maintain multiple implementations
- Full AWS integration requires additional setup

## Implementation

### Interface Design

```go
type SecretStore interface {
    GetSecret(ctx context.Context, name string) (string, error)
}

type MessageQueue interface {
    Publish(ctx context.Context, msg Message) error
    Subscribe(ctx context.Context) (<-chan Message, error)
}

type NotificationService interface {
    Notify(ctx context.Context, topic string, msg Notification) error
}
```

### Implementations

| Interface | Production | Development |
|-----------|------------|-------------|
| SecretStore | AWS Secrets Manager | Environment variables |
| MessageQueue | AWS SQS | In-memory channel |
| NotificationService | AWS SNS | Logging only |

### Configuration

```yaml
# Development (default)
aws:
  enabled: false

# Production
aws:
  enabled: true
  region: us-east-1
  secrets_name: aigateway/providers
  sqs_queue_url: https://sqs.us-east-1.amazonaws.com/...
  sns_topic_arn: arn:aws:sns:us-east-1:...
```

## Alternatives Considered

### Require AWS Always

- Higher barrier to entry
- Costs money for development
- Slower feedback loop

### No AWS Integration

- Limits production deployment options
- Less cloud-native

### LocalStack Only

- Good middle ground but adds Docker complexity
- Included as option, not requirement
