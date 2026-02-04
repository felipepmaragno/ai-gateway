package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/cost"
)

type PostgresUsageRepository struct {
	db *sql.DB
}

func NewPostgresUsageRepository(db *sql.DB) *PostgresUsageRepository {
	return &PostgresUsageRepository{db: db}
}

func (r *PostgresUsageRepository) Record(ctx context.Context, record cost.UsageRecord) error {
	query := `
		INSERT INTO usage_records (tenant_id, request_id, model, provider, input_tokens, output_tokens, cost_usd, cached, latency_ms, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		record.TenantID,
		record.RequestID,
		record.Model,
		record.Provider,
		record.InputTokens,
		record.OutputTokens,
		record.CostUSD,
		record.Cached,
		record.LatencyMs,
		"success",
		record.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("insert usage record: %w", err)
	}

	return nil
}

func (r *PostgresUsageRepository) GetTenantUsage(ctx context.Context, tenantID string, since time.Time) ([]cost.UsageRecord, error) {
	query := `
		SELECT tenant_id, request_id, model, provider, input_tokens, output_tokens, cost_usd, created_at
		FROM usage_records
		WHERE tenant_id = $1 AND created_at >= $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID, since)
	if err != nil {
		return nil, fmt.Errorf("query usage records: %w", err)
	}
	defer rows.Close()

	var records []cost.UsageRecord
	for rows.Next() {
		var record cost.UsageRecord
		err := rows.Scan(
			&record.TenantID,
			&record.RequestID,
			&record.Model,
			&record.Provider,
			&record.InputTokens,
			&record.OutputTokens,
			&record.CostUSD,
			&record.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("scan usage record: %w", err)
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

func (r *PostgresUsageRepository) GetTenantTotalCost(ctx context.Context, tenantID string, since time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(cost_usd), 0)
		FROM usage_records
		WHERE tenant_id = $1 AND created_at >= $2
	`

	var total float64
	err := r.db.QueryRowContext(ctx, query, tenantID, since).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("query total cost: %w", err)
	}

	return total, nil
}
