package budget

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// AlertDeduplicator handles deduplication of budget alerts across instances.
// It ensures that the same alert is not sent multiple times when running
// multiple gateway instances.
type AlertDeduplicator interface {
	// ShouldAlert checks if an alert should be sent for the given tenant and level.
	// Returns true if this is a new alert that should be dispatched.
	// Returns false if this alert was already sent (by this or another instance).
	ShouldAlert(ctx context.Context, tenantID string, level AlertLevel) bool

	// ClearAlert removes the alert state for a tenant (e.g., when usage drops below threshold).
	ClearAlert(ctx context.Context, tenantID string)
}

// InMemoryDeduplicator implements AlertDeduplicator using in-memory state.
// Suitable for single-instance deployments.
type InMemoryDeduplicator struct {
	mu         sync.RWMutex
	lastAlerts map[string]AlertLevel
}

// NewInMemoryDeduplicator creates a new in-memory alert deduplicator.
func NewInMemoryDeduplicator() *InMemoryDeduplicator {
	return &InMemoryDeduplicator{
		lastAlerts: make(map[string]AlertLevel),
	}
}

func (d *InMemoryDeduplicator) ShouldAlert(ctx context.Context, tenantID string, level AlertLevel) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	lastLevel, exists := d.lastAlerts[tenantID]
	if exists && lastLevel == level {
		return false
	}

	d.lastAlerts[tenantID] = level
	return true
}

func (d *InMemoryDeduplicator) ClearAlert(ctx context.Context, tenantID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.lastAlerts, tenantID)
}

// RedisDeduplicator implements AlertDeduplicator using Redis for distributed state.
// Ensures alert deduplication across multiple gateway instances.
type RedisDeduplicator struct {
	client  *redis.Client
	lockTTL time.Duration
}

// NewRedisDeduplicator creates a new Redis-backed alert deduplicator.
// lockTTL determines how long an alert is considered "sent" before it can be re-sent.
// Recommended: 1 hour for budget alerts (they reset monthly).
func NewRedisDeduplicator(redisURL string, lockTTL time.Duration) (*RedisDeduplicator, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &RedisDeduplicator{
		client:  client,
		lockTTL: lockTTL,
	}, nil
}

// NewRedisDeduplicatorWithClient creates a deduplicator with an existing Redis client.
func NewRedisDeduplicatorWithClient(client *redis.Client, lockTTL time.Duration) *RedisDeduplicator {
	return &RedisDeduplicator{
		client:  client,
		lockTTL: lockTTL,
	}
}

func (d *RedisDeduplicator) alertKey(tenantID string, level AlertLevel) string {
	return fmt.Sprintf("budget:alert:%s:%s", tenantID, level)
}

func (d *RedisDeduplicator) tenantKeyPattern(tenantID string) string {
	return fmt.Sprintf("budget:alert:%s:*", tenantID)
}

// ShouldAlert uses Redis SETNX for atomic check-and-set.
// Only one instance will successfully set the key and return true.
func (d *RedisDeduplicator) ShouldAlert(ctx context.Context, tenantID string, level AlertLevel) bool {
	key := d.alertKey(tenantID, level)

	// Try to acquire the "lock" for this alert
	// SETNX returns true only if the key didn't exist
	acquired, err := d.client.SetNX(ctx, key, time.Now().Unix(), d.lockTTL).Result()
	if err != nil {
		// On Redis error, allow the alert (fail open)
		return true
	}

	return acquired
}

// ClearAlert removes all alert keys for a tenant.
// Called when usage drops below warning threshold.
func (d *RedisDeduplicator) ClearAlert(ctx context.Context, tenantID string) {
	// Find all alert keys for this tenant
	pattern := d.tenantKeyPattern(tenantID)
	keys, err := d.client.Keys(ctx, pattern).Result()
	if err != nil || len(keys) == 0 {
		return
	}

	// Delete all found keys
	d.client.Del(ctx, keys...)
}

// Close closes the Redis connection.
func (d *RedisDeduplicator) Close() error {
	return d.client.Close()
}
