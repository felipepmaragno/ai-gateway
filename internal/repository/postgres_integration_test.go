//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/cost"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/felipepmaragno/ai-gateway/internal/repository"
	_ "github.com/lib/pq"
)

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	return db
}

func TestPostgresTenantRepository_CRUD(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	repo := repository.NewPostgresTenantRepository(db)
	ctx := context.Background()

	tenant := &domain.Tenant{
		ID:           "test-tenant-" + time.Now().Format("20060102150405"),
		Name:         "Test Tenant",
		APIKey:       "gw-test-key-123",
		APIKeyHash:   "hash123",
		BudgetUSD:    100.0,
		RateLimitRPM: 60,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.Name != tenant.Name {
		t.Errorf("expected name %s, got %s", tenant.Name, got.Name)
	}

	tenant.Name = "Updated Tenant"
	if err := repo.Update(ctx, tenant); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err = repo.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}

	if got.Name != "Updated Tenant" {
		t.Errorf("expected updated name, got %s", got.Name)
	}

	tenants, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, ten := range tenants {
		if ten.ID == tenant.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("tenant not found in list")
	}

	if err := repo.Delete(ctx, tenant.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.GetByID(ctx, tenant.ID)
	if err != domain.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound after delete, got %v", err)
	}
}

func TestPostgresUsageRepository_Record(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	tenantRepo := repository.NewPostgresTenantRepository(db)
	usageRepo := repository.NewPostgresUsageRepository(db)
	ctx := context.Background()

	tenant := &domain.Tenant{
		ID:           "usage-test-tenant-" + time.Now().Format("20060102150405"),
		Name:         "Usage Test Tenant",
		APIKeyHash:   "usagehash123",
		BudgetUSD:    100.0,
		RateLimitRPM: 60,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := tenantRepo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create tenant failed: %v", err)
	}
	defer tenantRepo.Delete(ctx, tenant.ID)

	record := cost.UsageRecord{
		TenantID:     tenant.ID,
		RequestID:    "req-123",
		Model:        "gpt-4",
		Provider:     "openai",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.01,
		Timestamp:    time.Now(),
	}

	if err := usageRepo.Record(ctx, record); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	since := time.Now().Add(-1 * time.Hour)
	records, err := usageRepo.GetTenantUsage(ctx, tenant.ID, since)
	if err != nil {
		t.Fatalf("GetTenantUsage failed: %v", err)
	}

	if len(records) == 0 {
		t.Error("expected at least one usage record")
	}

	totalCost, err := usageRepo.GetTenantTotalCost(ctx, tenant.ID, since)
	if err != nil {
		t.Fatalf("GetTenantTotalCost failed: %v", err)
	}

	if totalCost < 0.01 {
		t.Errorf("expected total cost >= 0.01, got %f", totalCost)
	}
}
