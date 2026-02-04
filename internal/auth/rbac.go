package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

type AdminUser struct {
	ID           string
	Username     string
	PasswordHash string
	Role         Role
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Permission string

const (
	PermissionTenantRead   Permission = "tenant:read"
	PermissionTenantWrite  Permission = "tenant:write"
	PermissionTenantDelete Permission = "tenant:delete"
	PermissionUsageRead    Permission = "usage:read"
	PermissionAdminManage  Permission = "admin:manage"
)

var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermissionTenantRead,
		PermissionTenantWrite,
		PermissionTenantDelete,
		PermissionUsageRead,
		PermissionAdminManage,
	},
	RoleEditor: {
		PermissionTenantRead,
		PermissionTenantWrite,
		PermissionUsageRead,
	},
	RoleViewer: {
		PermissionTenantRead,
		PermissionUsageRead,
	},
}

func HasPermission(role Role, permission Permission) bool {
	permissions, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range permissions {
		if p == permission {
			return true
		}
	}
	return false
}

type AdminUserRepository interface {
	GetByUsername(ctx context.Context, username string) (*AdminUser, error)
	GetByID(ctx context.Context, id string) (*AdminUser, error)
	Create(ctx context.Context, user *AdminUser) error
	Update(ctx context.Context, user *AdminUser) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*AdminUser, error)
}

type Authenticator struct {
	repo AdminUserRepository
}

func NewAuthenticator(repo AdminUserRepository) *Authenticator {
	return &Authenticator{repo: repo}
}

func (a *Authenticator) Authenticate(ctx context.Context, username, password string) (*AdminUser, error) {
	user, err := a.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if !user.Enabled {
		return nil, ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	return user, nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

type contextKey string

const userContextKey contextKey = "admin_user"

func WithUser(ctx context.Context, user *AdminUser) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*AdminUser, bool) {
	user, ok := ctx.Value(userContextKey).(*AdminUser)
	return user, ok
}

type RBACMiddleware struct {
	auth *Authenticator
}

func NewRBACMiddleware(auth *Authenticator) *RBACMiddleware {
	return &RBACMiddleware{auth: auth}
}

func (m *RBACMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Admin API"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := m.auth.Authenticate(r.Context(), username, password)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := WithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *RBACMiddleware) RequirePermission(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !HasPermission(user.Role, permission) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type PostgresAdminUserRepository struct {
	db *sql.DB
}

func NewPostgresAdminUserRepository(db *sql.DB) *PostgresAdminUserRepository {
	return &PostgresAdminUserRepository{db: db}
}

func (r *PostgresAdminUserRepository) GetByUsername(ctx context.Context, username string) (*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, role, enabled, created_at, updated_at
		FROM admin_users
		WHERE username = $1
	`

	var user AdminUser
	var role string
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&role,
		&user.Enabled,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	user.Role = Role(role)
	return &user, nil
}

func (r *PostgresAdminUserRepository) GetByID(ctx context.Context, id string) (*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, role, enabled, created_at, updated_at
		FROM admin_users
		WHERE id = $1
	`

	var user AdminUser
	var role string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&role,
		&user.Enabled,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	user.Role = Role(role)
	return &user, nil
}

func (r *PostgresAdminUserRepository) Create(ctx context.Context, user *AdminUser) error {
	query := `
		INSERT INTO admin_users (id, username, password_hash, role, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Username,
		user.PasswordHash,
		string(user.Role),
		user.Enabled,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

func (r *PostgresAdminUserRepository) Update(ctx context.Context, user *AdminUser) error {
	query := `
		UPDATE admin_users
		SET username = $2, password_hash = $3, role = $4, enabled = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Username,
		user.PasswordHash,
		string(user.Role),
		user.Enabled,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *PostgresAdminUserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM admin_users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *PostgresAdminUserRepository) List(ctx context.Context) ([]*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, role, enabled, created_at, updated_at
		FROM admin_users
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []*AdminUser
	for rows.Next() {
		var user AdminUser
		var role string
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&role,
			&user.Enabled,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		user.Role = Role(role)
		users = append(users, &user)
	}

	return users, rows.Err()
}

type InMemoryAdminUserRepository struct {
	users map[string]*AdminUser
}

func NewInMemoryAdminUserRepository() *InMemoryAdminUserRepository {
	repo := &InMemoryAdminUserRepository{
		users: make(map[string]*AdminUser),
	}

	adminHash, _ := HashPassword("admin")
	repo.users["admin"] = &AdminUser{
		ID:           "admin",
		Username:     "admin",
		PasswordHash: adminHash,
		Role:         RoleAdmin,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return repo
}

func (r *InMemoryAdminUserRepository) GetByUsername(ctx context.Context, username string) (*AdminUser, error) {
	for _, u := range r.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (r *InMemoryAdminUserRepository) GetByID(ctx context.Context, id string) (*AdminUser, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (r *InMemoryAdminUserRepository) Create(ctx context.Context, user *AdminUser) error {
	r.users[user.ID] = user
	return nil
}

func (r *InMemoryAdminUserRepository) Update(ctx context.Context, user *AdminUser) error {
	if _, ok := r.users[user.ID]; !ok {
		return ErrUserNotFound
	}
	r.users[user.ID] = user
	return nil
}

func (r *InMemoryAdminUserRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.users[id]; !ok {
		return ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *InMemoryAdminUserRepository) List(ctx context.Context) ([]*AdminUser, error) {
	users := make([]*AdminUser, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, u)
	}
	return users, nil
}

func GenerateAPIToken(userID string) string {
	data := fmt.Sprintf("%s:%d", userID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func ExtractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
