# Auth Package

Authentication and authorization for the Admin API.

## Overview

Implements Role-Based Access Control (RBAC) for administrative operations.

## Components

### Basic Authentication

Admin API uses HTTP Basic Auth:
```
Authorization: Basic base64(username:password)
```

### Roles

| Role | Permissions |
|------|-------------|
| `admin` | Full access (CRUD on all resources) |
| `editor` | Create, read, update (no delete) |
| `viewer` | Read-only access |

### Permissions

```go
const (
    PermissionRead   = "read"
    PermissionCreate = "create"
    PermissionUpdate = "update"
    PermissionDelete = "delete"
)
```

## RBAC Interface

```go
type RBAC interface {
    HasPermission(role string, permission string) bool
    GetPermissions(role string) []string
}
```

## Usage

```go
rbac := auth.NewRBAC()

// Check permission
if !rbac.HasPermission(user.Role, auth.PermissionDelete) {
    return ErrForbidden
}
```

## Middleware

```go
func RequireAuth(rbac *RBAC, permission string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := authenticate(r)
            if !rbac.HasPermission(user.Role, permission) {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

## Configuration

Enable/disable via environment:
```
ADMIN_AUTH_ENABLED=true
```

Default credentials (change in production!):
```
Username: admin
Password: admin
```

## Dependencies

- `internal/domain` - User types
- `internal/repository` - Admin user storage
