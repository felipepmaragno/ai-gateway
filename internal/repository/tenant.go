package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type TenantRepository interface {
	GetByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error)
	GetByID(ctx context.Context, id string) (*domain.Tenant, error)
	Create(ctx context.Context, tenant *domain.Tenant) error
	Update(ctx context.Context, tenant *domain.Tenant) error
}

type InMemoryTenantRepository struct {
	mu      sync.RWMutex
	tenants map[string]*domain.Tenant
	byKey   map[string]string
}

func NewInMemoryTenantRepository() *InMemoryTenantRepository {
	repo := &InMemoryTenantRepository{
		tenants: make(map[string]*domain.Tenant),
		byKey:   make(map[string]string),
	}

	defaultTenant := &domain.Tenant{
		ID:                "default",
		Name:              "default",
		APIKeyHash:        hashAPIKey("gw-default-key"),
		BudgetUSD:         1000.0,
		RateLimitRPM:      100,
		AllowedModels:     []string{},
		DefaultProvider:   "ollama",
		FallbackProviders: []string{},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	repo.tenants[defaultTenant.ID] = defaultTenant
	repo.byKey[defaultTenant.APIKeyHash] = defaultTenant.ID

	return repo
}

func (r *InMemoryTenantRepository) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hash := hashAPIKey(apiKey)
	tenantID, ok := r.byKey[hash]
	if !ok {
		return nil, domain.ErrTenantNotFound
	}

	tenant, ok := r.tenants[tenantID]
	if !ok {
		return nil, domain.ErrTenantNotFound
	}

	return tenant, nil
}

func (r *InMemoryTenantRepository) GetByID(ctx context.Context, id string) (*domain.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenant, ok := r.tenants[id]
	if !ok {
		return nil, domain.ErrTenantNotFound
	}

	return tenant, nil
}

func (r *InMemoryTenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tenants[tenant.ID] = tenant
	r.byKey[tenant.APIKeyHash] = tenant.ID

	return nil
}

func (r *InMemoryTenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tenants[tenant.ID]; !ok {
		return domain.ErrTenantNotFound
	}

	tenant.UpdatedAt = time.Now()
	r.tenants[tenant.ID] = tenant

	return nil
}

func hashAPIKey(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(h[:])
}
