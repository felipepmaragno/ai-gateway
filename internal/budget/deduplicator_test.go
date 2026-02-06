package budget

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func TestInMemoryDeduplicator_ShouldAlert(t *testing.T) {
	ctx := context.Background()
	d := NewInMemoryDeduplicator()

	// First alert should be allowed
	if !d.ShouldAlert(ctx, "tenant1", AlertLevelWarning) {
		t.Error("First alert should be allowed")
	}

	// Same alert should be deduplicated
	if d.ShouldAlert(ctx, "tenant1", AlertLevelWarning) {
		t.Error("Same alert should be deduplicated")
	}

	// Different level should be allowed
	if !d.ShouldAlert(ctx, "tenant1", AlertLevelCritical) {
		t.Error("Different level should be allowed")
	}

	// Different tenant should be allowed
	if !d.ShouldAlert(ctx, "tenant2", AlertLevelWarning) {
		t.Error("Different tenant should be allowed")
	}
}

func TestInMemoryDeduplicator_ClearAlert(t *testing.T) {
	ctx := context.Background()
	d := NewInMemoryDeduplicator()

	// Set an alert
	d.ShouldAlert(ctx, "tenant1", AlertLevelWarning)

	// Clear it
	d.ClearAlert(ctx, "tenant1")

	// Should be able to alert again
	if !d.ShouldAlert(ctx, "tenant1", AlertLevelWarning) {
		t.Error("After clear, should be able to alert again")
	}
}

func getRedisURL(t *testing.T) string {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set, skipping Redis deduplicator tests")
	}
	return url
}

func TestRedisDeduplicator_ShouldAlert(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	d, err := NewRedisDeduplicator(redisURL, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create Redis deduplicator: %v", err)
	}
	defer d.Close()
	defer d.ClearAlert(ctx, "redis-tenant1")

	// First alert should be allowed
	if !d.ShouldAlert(ctx, "redis-tenant1", AlertLevelWarning) {
		t.Error("First alert should be allowed")
	}

	// Same alert should be deduplicated
	if d.ShouldAlert(ctx, "redis-tenant1", AlertLevelWarning) {
		t.Error("Same alert should be deduplicated")
	}

	// Different level should be allowed
	if !d.ShouldAlert(ctx, "redis-tenant1", AlertLevelCritical) {
		t.Error("Different level should be allowed")
	}
}

func TestRedisDeduplicator_ClearAlert(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	d, err := NewRedisDeduplicator(redisURL, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create Redis deduplicator: %v", err)
	}
	defer d.Close()

	// Set alerts at multiple levels
	d.ShouldAlert(ctx, "redis-tenant2", AlertLevelWarning)
	d.ShouldAlert(ctx, "redis-tenant2", AlertLevelCritical)

	// Clear all alerts for tenant
	d.ClearAlert(ctx, "redis-tenant2")

	// Should be able to alert again at both levels
	if !d.ShouldAlert(ctx, "redis-tenant2", AlertLevelWarning) {
		t.Error("After clear, should be able to alert warning again")
	}
	if !d.ShouldAlert(ctx, "redis-tenant2", AlertLevelCritical) {
		t.Error("After clear, should be able to alert critical again")
	}
}

func TestRedisDeduplicator_TTLExpiry(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	// Use very short TTL for testing
	d, err := NewRedisDeduplicator(redisURL, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create Redis deduplicator: %v", err)
	}
	defer d.Close()

	// Set an alert
	if !d.ShouldAlert(ctx, "redis-tenant3", AlertLevelWarning) {
		t.Error("First alert should be allowed")
	}

	// Should be deduplicated immediately
	if d.ShouldAlert(ctx, "redis-tenant3", AlertLevelWarning) {
		t.Error("Same alert should be deduplicated")
	}

	// Wait for TTL to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be able to alert again after TTL
	if !d.ShouldAlert(ctx, "redis-tenant3", AlertLevelWarning) {
		t.Error("After TTL expiry, should be able to alert again")
	}
}

func TestMonitor_WithRedisDeduplicator(t *testing.T) {
	redisURL := getRedisURL(t)
	ctx := context.Background()

	tracker := newMockTracker()
	tracker.costs["tenant1"] = 85.0

	dedup, err := NewRedisDeduplicator(redisURL, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create Redis deduplicator: %v", err)
	}
	defer dedup.Close()
	defer dedup.ClearAlert(ctx, "tenant1")

	monitor := NewMonitor(tracker, DefaultThresholds(), WithDeduplicator(dedup))

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 100.0,
	}

	// First check should return alert
	alert1, err := monitor.Check(ctx, tenant)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if alert1 == nil {
		t.Fatal("First check should return alert")
	}

	// Second check should be deduplicated
	alert2, err := monitor.Check(ctx, tenant)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if alert2 != nil {
		t.Error("Second check should be deduplicated")
	}
}
