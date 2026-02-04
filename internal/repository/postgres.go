package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/lib/pq"
)

type PostgresTenantRepository struct {
	db *sql.DB
}

func NewPostgresTenantRepository(db *sql.DB) *PostgresTenantRepository {
	return &PostgresTenantRepository{db: db}
}

func (r *PostgresTenantRepository) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error) {
	hash := hashAPIKey(apiKey)

	query := `
		SELECT id, name, api_key_hash, budget_usd, rate_limit_rpm, 
		       allowed_models, default_provider, fallback_providers, enabled, created_at, updated_at
		FROM tenants
		WHERE api_key_hash = $1 AND enabled = true
	`

	var tenant domain.Tenant
	var allowedModels, fallbackProviders pq.StringArray
	var defaultProvider sql.NullString

	err := r.db.QueryRowContext(ctx, query, hash).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.APIKeyHash,
		&tenant.BudgetUSD,
		&tenant.RateLimitRPM,
		&allowedModels,
		&defaultProvider,
		&fallbackProviders,
		&tenant.Enabled,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrTenantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query tenant: %w", err)
	}

	tenant.AllowedModels = []string(allowedModels)
	tenant.FallbackProviders = []string(fallbackProviders)
	if defaultProvider.Valid {
		tenant.DefaultProvider = defaultProvider.String
	}

	return &tenant, nil
}

func (r *PostgresTenantRepository) GetByID(ctx context.Context, id string) (*domain.Tenant, error) {
	query := `
		SELECT id, name, api_key_hash, budget_usd, rate_limit_rpm, 
		       allowed_models, default_provider, fallback_providers, enabled, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`

	var tenant domain.Tenant
	var allowedModels, fallbackProviders pq.StringArray
	var defaultProvider sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.APIKeyHash,
		&tenant.BudgetUSD,
		&tenant.RateLimitRPM,
		&allowedModels,
		&defaultProvider,
		&fallbackProviders,
		&tenant.Enabled,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrTenantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query tenant: %w", err)
	}

	tenant.AllowedModels = []string(allowedModels)
	tenant.FallbackProviders = []string(fallbackProviders)
	if defaultProvider.Valid {
		tenant.DefaultProvider = defaultProvider.String
	}

	return &tenant, nil
}

func (r *PostgresTenantRepository) List(ctx context.Context) ([]*domain.Tenant, error) {
	query := `
		SELECT id, name, api_key_hash, budget_usd, rate_limit_rpm, 
		       allowed_models, default_provider, fallback_providers, enabled, created_at, updated_at
		FROM tenants
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*domain.Tenant
	for rows.Next() {
		var tenant domain.Tenant
		var allowedModels, fallbackProviders pq.StringArray
		var defaultProvider sql.NullString

		err := rows.Scan(
			&tenant.ID,
			&tenant.Name,
			&tenant.APIKeyHash,
			&tenant.BudgetUSD,
			&tenant.RateLimitRPM,
			&allowedModels,
			&defaultProvider,
			&fallbackProviders,
			&tenant.Enabled,
			&tenant.CreatedAt,
			&tenant.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}

		tenant.AllowedModels = []string(allowedModels)
		tenant.FallbackProviders = []string(fallbackProviders)
		if defaultProvider.Valid {
			tenant.DefaultProvider = defaultProvider.String
		}

		tenants = append(tenants, &tenant)
	}

	return tenants, rows.Err()
}

func (r *PostgresTenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	query := `
		INSERT INTO tenants (id, name, api_key_hash, budget_usd, rate_limit_rpm, 
		                     allowed_models, default_provider, fallback_providers, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		tenant.ID,
		tenant.Name,
		tenant.APIKeyHash,
		tenant.BudgetUSD,
		tenant.RateLimitRPM,
		pq.Array(tenant.AllowedModels),
		sql.NullString{String: tenant.DefaultProvider, Valid: tenant.DefaultProvider != ""},
		pq.Array(tenant.FallbackProviders),
		tenant.Enabled,
		tenant.CreatedAt,
		tenant.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}

	return nil
}

func (r *PostgresTenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	query := `
		UPDATE tenants
		SET name = $2, api_key_hash = $3, budget_usd = $4, rate_limit_rpm = $5,
		    allowed_models = $6, default_provider = $7, fallback_providers = $8, 
		    enabled = $9, updated_at = $10
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		tenant.ID,
		tenant.Name,
		tenant.APIKeyHash,
		tenant.BudgetUSD,
		tenant.RateLimitRPM,
		pq.Array(tenant.AllowedModels),
		sql.NullString{String: tenant.DefaultProvider, Valid: tenant.DefaultProvider != ""},
		pq.Array(tenant.FallbackProviders),
		tenant.Enabled,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTenantNotFound
	}

	return nil
}

func (r *PostgresTenantRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tenants WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTenantNotFound
	}

	return nil
}
