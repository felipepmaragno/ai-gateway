package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       Role
		permission Permission
		want       bool
	}{
		// Admin has all permissions
		{"admin tenant:read", RoleAdmin, PermissionTenantRead, true},
		{"admin tenant:write", RoleAdmin, PermissionTenantWrite, true},
		{"admin tenant:delete", RoleAdmin, PermissionTenantDelete, true},
		{"admin usage:read", RoleAdmin, PermissionUsageRead, true},
		{"admin admin:manage", RoleAdmin, PermissionAdminManage, true},

		// Editor has read/write but not delete/admin
		{"editor tenant:read", RoleEditor, PermissionTenantRead, true},
		{"editor tenant:write", RoleEditor, PermissionTenantWrite, true},
		{"editor tenant:delete", RoleEditor, PermissionTenantDelete, false},
		{"editor usage:read", RoleEditor, PermissionUsageRead, true},
		{"editor admin:manage", RoleEditor, PermissionAdminManage, false},

		// Viewer has read only
		{"viewer tenant:read", RoleViewer, PermissionTenantRead, true},
		{"viewer tenant:write", RoleViewer, PermissionTenantWrite, false},
		{"viewer tenant:delete", RoleViewer, PermissionTenantDelete, false},
		{"viewer usage:read", RoleViewer, PermissionUsageRead, true},
		{"viewer admin:manage", RoleViewer, PermissionAdminManage, false},

		// Unknown role has no permissions
		{"unknown role", Role("unknown"), PermissionTenantRead, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.role, tt.permission); got != tt.want {
				t.Errorf("HasPermission(%v, %v) = %v, want %v", tt.role, tt.permission, got, tt.want)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	password := "test-password-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Hash should not be empty
	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}

	// Hash should not equal password
	if hash == password {
		t.Error("HashPassword() returned unhashed password")
	}

	// Different calls should produce different hashes (bcrypt uses random salt)
	hash2, _ := HashPassword(password)
	if hash == hash2 {
		t.Error("HashPassword() should produce different hashes due to random salt")
	}
}

func TestAuthenticator_Authenticate(t *testing.T) {
	repo := NewInMemoryAdminUserRepository()
	auth := NewAuthenticator(repo)

	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{"valid credentials", "admin", "admin", nil},
		{"wrong password", "admin", "wrong", ErrInvalidPassword},
		{"unknown user", "unknown", "password", ErrUserNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := auth.Authenticate(context.Background(), tt.username, tt.password)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Authenticate() unexpected error = %v", err)
				return
			}

			if user.Username != tt.username {
				t.Errorf("Authenticate() user.Username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}

func TestAuthenticator_DisabledUser(t *testing.T) {
	repo := NewInMemoryAdminUserRepository()

	// Create disabled user
	hash, _ := HashPassword("password")
	repo.Create(context.Background(), &AdminUser{
		ID:           "disabled-user",
		Username:     "disabled",
		PasswordHash: hash,
		Role:         RoleViewer,
		Enabled:      false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	auth := NewAuthenticator(repo)

	_, err := auth.Authenticate(context.Background(), "disabled", "password")
	if err != ErrUnauthorized {
		t.Errorf("Authenticate() disabled user error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestUserContext(t *testing.T) {
	user := &AdminUser{
		ID:       "test-id",
		Username: "testuser",
		Role:     RoleAdmin,
	}

	ctx := context.Background()

	// No user in context
	_, ok := UserFromContext(ctx)
	if ok {
		t.Error("UserFromContext() should return false for empty context")
	}

	// Add user to context
	ctx = WithUser(ctx, user)
	gotUser, ok := UserFromContext(ctx)
	if !ok {
		t.Error("UserFromContext() should return true after WithUser")
	}
	if gotUser.ID != user.ID {
		t.Errorf("UserFromContext() user.ID = %v, want %v", gotUser.ID, user.ID)
	}
}

func TestRBACMiddleware_RequireAuth(t *testing.T) {
	repo := NewInMemoryAdminUserRepository()
	auth := NewAuthenticator(repo)
	middleware := NewRBACMiddleware(auth)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("User should be in context after auth")
		}
		if user.Username != "admin" {
			t.Errorf("Username = %v, want admin", user.Username)
		}
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		{"valid auth", "admin", "admin", http.StatusOK},
		{"wrong password", "admin", "wrong", http.StatusUnauthorized},
		{"no auth", "", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/admin/test", nil)
			if tt.username != "" {
				req.SetBasicAuth(tt.username, tt.password)
			}

			rr := httptest.NewRecorder()
			middleware.RequireAuth(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("RequireAuth() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestRBACMiddleware_RequirePermission(t *testing.T) {
	middleware := &RBACMiddleware{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		role       Role
		permission Permission
		wantStatus int
	}{
		{"admin with delete", RoleAdmin, PermissionTenantDelete, http.StatusOK},
		{"editor without delete", RoleEditor, PermissionTenantDelete, http.StatusForbidden},
		{"viewer without write", RoleViewer, PermissionTenantWrite, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &AdminUser{ID: "test", Username: "test", Role: tt.role}
			req := httptest.NewRequest("GET", "/test", nil)
			req = req.WithContext(WithUser(req.Context(), user))

			rr := httptest.NewRecorder()
			middleware.RequirePermission(tt.permission)(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("RequirePermission() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestRBACMiddleware_RequirePermission_NoUser(t *testing.T) {
	middleware := &RBACMiddleware{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	middleware.RequirePermission(PermissionTenantRead)(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("RequirePermission() without user status = %v, want %v", rr.Code, http.StatusUnauthorized)
	}
}

func TestInMemoryAdminUserRepository(t *testing.T) {
	repo := NewInMemoryAdminUserRepository()
	ctx := context.Background()

	// Should have default admin user
	user, err := repo.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetByUsername(admin) error = %v", err)
	}
	if user.Role != RoleAdmin {
		t.Errorf("Default admin role = %v, want %v", user.Role, RoleAdmin)
	}

	// Create new user
	newUser := &AdminUser{
		ID:           "new-user",
		Username:     "newuser",
		PasswordHash: "hash",
		Role:         RoleViewer,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, newUser); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get by ID
	got, err := repo.GetByID(ctx, "new-user")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Username != "newuser" {
		t.Errorf("GetByID() username = %v, want newuser", got.Username)
	}

	// Update
	newUser.Role = RoleEditor
	if err := repo.Update(ctx, newUser); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	got, _ = repo.GetByID(ctx, "new-user")
	if got.Role != RoleEditor {
		t.Errorf("Update() role = %v, want %v", got.Role, RoleEditor)
	}

	// List
	users, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 2 {
		t.Errorf("List() count = %v, want 2", len(users))
	}

	// Delete
	if err := repo.Delete(ctx, "new-user"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = repo.GetByID(ctx, "new-user")
	if err != ErrUserNotFound {
		t.Errorf("GetByID after delete error = %v, want %v", err, ErrUserNotFound)
	}

	// Delete non-existent
	err = repo.Delete(ctx, "non-existent")
	if err != ErrUserNotFound {
		t.Errorf("Delete non-existent error = %v, want %v", err, ErrUserNotFound)
	}

	// Update non-existent
	err = repo.Update(ctx, &AdminUser{ID: "non-existent"})
	if err != ErrUserNotFound {
		t.Errorf("Update non-existent error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestGenerateAPIToken(t *testing.T) {
	token1 := GenerateAPIToken("user1")
	token2 := GenerateAPIToken("user1")

	// Should be 64 hex chars (SHA-256)
	if len(token1) != 64 {
		t.Errorf("GenerateAPIToken length = %d, want 64", len(token1))
	}

	// Different calls should produce different tokens (includes timestamp)
	if token1 == token2 {
		t.Error("GenerateAPIToken should produce different tokens")
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer", "Bearer abc123", "abc123"},
		{"no bearer prefix", "abc123", ""},
		{"empty header", "", ""},
		{"basic auth", "Basic dXNlcjpwYXNz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := ExtractBearerToken(req)
			if got != tt.want {
				t.Errorf("ExtractBearerToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
