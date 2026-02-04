package repository

import (
	"context"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func TestInMemoryTenantRepository_GetByAPIKey(t *testing.T) {
	repo := NewInMemoryTenantRepository()
	ctx := context.Background()

	tenant, err := repo.GetByAPIKey(ctx, "gw-default-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.ID != "default" {
		t.Errorf("expected tenant ID 'default', got %s", tenant.ID)
	}
}

func TestInMemoryTenantRepository_GetByAPIKey_NotFound(t *testing.T) {
	repo := NewInMemoryTenantRepository()
	ctx := context.Background()

	_, err := repo.GetByAPIKey(ctx, "invalid-key")
	if err != domain.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestInMemoryTenantRepository_Create(t *testing.T) {
	repo := NewInMemoryTenantRepository()
	ctx := context.Background()

	tenant := &domain.Tenant{
		ID:           "test-tenant",
		Name:         "Test Tenant",
		APIKeyHash:   hashAPIKey("test-key"),
		RateLimitRPM: 50,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.Create(ctx, tenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := repo.GetByAPIKey(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != "test-tenant" {
		t.Errorf("expected tenant ID 'test-tenant', got %s", retrieved.ID)
	}
}
