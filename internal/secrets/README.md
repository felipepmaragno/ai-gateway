# Secrets Package

Secure secret management with AWS Secrets Manager.

## Overview

Provides a unified interface for retrieving secrets with caching support.
Supports both AWS Secrets Manager (production) and in-memory (development).

## Backends

| Backend | Use Case | Caching |
|---------|----------|---------|
| AWS Secrets Manager | Production | Yes (5 min TTL) |
| In-Memory | Development, testing | No |

## Interface

```go
type SecretStore interface {
    GetSecret(ctx context.Context, name string) (string, error)
    GetSecretJSON(ctx context.Context, name string, v interface{}) error
}
```

## Usage

### AWS Secrets Manager

```go
store, err := secrets.NewAWSSecretsManager(ctx, "us-east-1")
if err != nil {
    log.Fatal(err)
}

// Get plain text secret
apiKey, err := store.GetSecret(ctx, "prod/openai-api-key")

// Get JSON secret
var creds struct {
    Username string `json:"username"`
    Password string `json:"password"`
}
err = store.GetSecretJSON(ctx, "prod/db-credentials", &creds)
```

### In-Memory (Testing)

```go
store := secrets.NewInMemorySecretStore()
store.SetSecret("test-key", "test-value")

value, err := store.GetSecret(ctx, "test-key")
```

## Caching

AWS Secrets Manager responses are cached to reduce API calls and latency:

```go
store.SetCacheTTL(10 * time.Minute)  // Change TTL
store.ClearCache()                    // Force refresh
```

Default TTL: 5 minutes

### Cache Behavior

```
Request → Cache Hit? → Yes → Return cached value
              ↓
             No
              ↓
        Call AWS API
              ↓
        Store in cache
              ↓
        Return value
```

## AWS Configuration

Required IAM permissions:
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetSecretValue"
            ],
            "Resource": "arn:aws:secretsmanager:*:*:secret:prod/*"
        }
    ]
}
```

Environment:
```bash
export AWS_REGION=us-east-1
# Credentials from environment, instance profile, or ~/.aws/credentials
```

## Secret Naming Convention

Recommended structure:
```
{environment}/{service}-{type}

Examples:
- prod/openai-api-key
- prod/anthropic-api-key
- prod/database-credentials
- staging/openai-api-key
```

## Use Cases

1. **API Keys**: Store provider API keys securely
2. **Database Credentials**: Rotate without redeployment
3. **Encryption Keys**: Manage encryption keys centrally
4. **Third-party Tokens**: Webhook secrets, OAuth tokens

## Error Handling

```go
secret, err := store.GetSecret(ctx, "my-secret")
if err != nil {
    // Could be:
    // - Secret not found
    // - Permission denied
    // - Network error
    // - Invalid JSON (for GetSecretJSON)
}
```

## Dependencies

- `github.com/aws/aws-sdk-go-v2/service/secretsmanager` - AWS SDK
