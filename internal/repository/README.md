# Repository Package

Data persistence layer for tenants and usage records.

## Overview

Provides storage backends for:
- Tenant management (CRUD operations)
- Usage records (token consumption, costs)

## Backends

| Backend | Use Case | Persistence |
|---------|----------|-------------|
| In-Memory | Development, testing | No |
| PostgreSQL | Production | Yes |

## Interfaces

### TenantRepository

```go
type TenantRepository interface {
    GetByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error)
    GetByID(ctx context.Context, id string) (*domain.Tenant, error)
    List(ctx context.Context) ([]*domain.Tenant, error)
    Create(ctx context.Context, tenant *domain.Tenant) error
    Update(ctx context.Context, tenant *domain.Tenant) error
    Delete(ctx context.Context, id string) error
}
```

### UsageRepository

```go
type UsageRepository interface {
    Record(ctx context.Context, record cost.UsageRecord) error
    GetTenantUsage(ctx context.Context, tenantID string, since time.Time) ([]cost.UsageRecord, error)
    GetTenantTotalCost(ctx context.Context, tenantID string, since time.Time) (float64, error)
}
```

## PostgreSQL Schema

See `migrations/001_initial.up.sql` for full schema.

Key tables:
- `tenants` - Tenant configuration and API keys
- `usage_records` - Request logs with token counts and costs
- `admin_users` - Admin API authentication

## API Key Security

API keys are stored securely:
- `api_key_hash` - SHA-256 hash for lookup (indexed)
- `api_key_encrypted` - AES-256-GCM encrypted for rotation/display

Lookup flow:
1. Hash incoming API key
2. Query by hash (fast, indexed)
3. Return tenant if found and enabled

## Connection Pooling

PostgreSQL connections are pooled:
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

## Usage Example

```go
// PostgreSQL
db, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))
tenantRepo := repository.NewPostgresTenantRepository(db)
usageRepo := repository.NewPostgresUsageRepository(db)

// In-Memory (testing)
tenantRepo := repository.NewInMemoryTenantRepository()
```

## Dependencies

- `internal/domain` - Tenant types
- `internal/cost` - UsageRecord type
- `github.com/lib/pq` - PostgreSQL driver
