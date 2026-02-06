package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// HealthChecker defines the interface for dependency health checks.
type HealthChecker interface {
	Check(ctx context.Context) error
	Name() string
}

// HealthCheckConfig configures the health check endpoints.
type HealthCheckConfig struct {
	Checkers []HealthChecker
	Timeout  time.Duration
}

// HealthStatus represents the result of a health check.
type HealthStatus struct {
	Status  string                 `json:"status"`
	Checks  map[string]CheckResult `json:"checks,omitempty"`
	Version string                 `json:"version,omitempty"`
}

// CheckResult represents the result of a single dependency check.
type CheckResult struct {
	Status   string `json:"status"`
	Duration string `json:"duration,omitempty"`
	Error    string `json:"error,omitempty"`
}

// RedisHealthChecker checks Redis connectivity.
type RedisHealthChecker struct {
	client *redis.Client
}

// NewRedisHealthChecker creates a health checker for Redis.
func NewRedisHealthChecker(redisURL string) (*RedisHealthChecker, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &RedisHealthChecker{client: redis.NewClient(opts)}, nil
}

// NewRedisHealthCheckerWithClient creates a health checker with an existing client.
func NewRedisHealthCheckerWithClient(client *redis.Client) *RedisHealthChecker {
	return &RedisHealthChecker{client: client}
}

func (c *RedisHealthChecker) Name() string {
	return "redis"
}

func (c *RedisHealthChecker) Check(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// PostgresHealthChecker checks PostgreSQL connectivity.
type PostgresHealthChecker struct {
	db *sql.DB
}

// NewPostgresHealthChecker creates a health checker for PostgreSQL.
func NewPostgresHealthChecker(db *sql.DB) *PostgresHealthChecker {
	return &PostgresHealthChecker{db: db}
}

func (c *PostgresHealthChecker) Name() string {
	return "postgres"
}

func (c *PostgresHealthChecker) Check(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// runHealthChecks executes all health checks concurrently.
func runHealthChecks(ctx context.Context, checkers []HealthChecker) map[string]CheckResult {
	results := make(map[string]CheckResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, checker := range checkers {
		wg.Add(1)
		go func(c HealthChecker) {
			defer wg.Done()

			start := time.Now()
			err := c.Check(ctx)
			duration := time.Since(start)

			result := CheckResult{
				Status:   "ok",
				Duration: duration.String(),
			}
			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
			}

			mu.Lock()
			results[c.Name()] = result
			mu.Unlock()
		}(checker)
	}

	wg.Wait()
	return results
}

// handleHealthReadyWithCheckers creates a ready handler with dependency checks.
func handleHealthReadyWithCheckers(checkers []HealthChecker, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		results := runHealthChecks(ctx, checkers)

		allHealthy := true
		for _, result := range results {
			if result.Status != "ok" {
				allHealthy = false
				break
			}
		}

		status := HealthStatus{
			Status:  "ready",
			Checks:  results,
			Version: "0.5.0",
		}

		httpStatus := http.StatusOK
		if !allHealthy {
			status.Status = "not_ready"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(status)
	}
}
