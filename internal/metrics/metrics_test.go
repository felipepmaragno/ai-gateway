package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordRequest(t *testing.T) {
	// Reset metrics for test isolation
	RequestsTotal.Reset()
	RequestDuration.Reset()

	RecordRequest("tenant1", "openai", "gpt-4", "success", 1.5)

	// Verify counter was incremented
	count := testutil.ToFloat64(RequestsTotal.WithLabelValues("tenant1", "openai", "gpt-4", "success"))
	if count != 1 {
		t.Errorf("RequestsTotal = %v, want 1", count)
	}
}

func TestRecordTokens(t *testing.T) {
	TokensTotal.Reset()

	RecordTokens("tenant1", "openai", "gpt-4", 100, 50)

	inputCount := testutil.ToFloat64(TokensTotal.WithLabelValues("tenant1", "openai", "gpt-4", "input"))
	if inputCount != 100 {
		t.Errorf("input tokens = %v, want 100", inputCount)
	}

	outputCount := testutil.ToFloat64(TokensTotal.WithLabelValues("tenant1", "openai", "gpt-4", "output"))
	if outputCount != 50 {
		t.Errorf("output tokens = %v, want 50", outputCount)
	}
}

func TestRecordCost(t *testing.T) {
	CostTotal.Reset()

	RecordCost("tenant1", "openai", "gpt-4", 0.05)
	RecordCost("tenant1", "openai", "gpt-4", 0.03)

	cost := testutil.ToFloat64(CostTotal.WithLabelValues("tenant1", "openai", "gpt-4"))
	if cost != 0.08 {
		t.Errorf("CostTotal = %v, want 0.08", cost)
	}
}

func TestRecordCacheHit(t *testing.T) {
	CacheHits.Reset()

	RecordCacheHit("tenant1")
	RecordCacheHit("tenant1")

	hits := testutil.ToFloat64(CacheHits.WithLabelValues("tenant1"))
	if hits != 2 {
		t.Errorf("CacheHits = %v, want 2", hits)
	}
}

func TestRecordCacheMiss(t *testing.T) {
	CacheMisses.Reset()

	RecordCacheMiss("tenant1")

	misses := testutil.ToFloat64(CacheMisses.WithLabelValues("tenant1"))
	if misses != 1 {
		t.Errorf("CacheMisses = %v, want 1", misses)
	}
}

func TestRecordProviderError(t *testing.T) {
	ProviderErrors.Reset()

	RecordProviderError("openai", "timeout")
	RecordProviderError("openai", "rate_limit")
	RecordProviderError("openai", "timeout")

	timeouts := testutil.ToFloat64(ProviderErrors.WithLabelValues("openai", "timeout"))
	if timeouts != 2 {
		t.Errorf("timeout errors = %v, want 2", timeouts)
	}

	rateLimits := testutil.ToFloat64(ProviderErrors.WithLabelValues("openai", "rate_limit"))
	if rateLimits != 1 {
		t.Errorf("rate_limit errors = %v, want 1", rateLimits)
	}
}

func TestRecordRateLimitHit(t *testing.T) {
	RateLimitHits.Reset()

	RecordRateLimitHit("tenant1")

	hits := testutil.ToFloat64(RateLimitHits.WithLabelValues("tenant1"))
	if hits != 1 {
		t.Errorf("RateLimitHits = %v, want 1", hits)
	}
}

func TestSetCircuitBreakerState(t *testing.T) {
	CircuitBreakerState.Reset()

	SetCircuitBreakerState("openai", 0) // closed
	state := testutil.ToFloat64(CircuitBreakerState.WithLabelValues("openai"))
	if state != 0 {
		t.Errorf("CircuitBreakerState = %v, want 0", state)
	}

	SetCircuitBreakerState("openai", 2) // open
	state = testutil.ToFloat64(CircuitBreakerState.WithLabelValues("openai"))
	if state != 2 {
		t.Errorf("CircuitBreakerState = %v, want 2", state)
	}
}

func TestSetBudgetUsage(t *testing.T) {
	BudgetUsageRatio.Reset()

	SetBudgetUsage("tenant1", 0.75)

	ratio := testutil.ToFloat64(BudgetUsageRatio.WithLabelValues("tenant1"))
	if ratio != 0.75 {
		t.Errorf("BudgetUsageRatio = %v, want 0.75", ratio)
	}
}

func TestActiveStreams(t *testing.T) {
	// Initialize instance metrics for testing
	InitInstanceMetrics("test-pod", "test-ns", "0.6.0")

	ActiveStreams.Reset()

	IncrementActiveStreams()
	IncrementActiveStreams()

	streams := testutil.ToFloat64(ActiveStreams.WithLabelValues("test-pod"))
	if streams != 2 {
		t.Errorf("ActiveStreams = %v, want 2", streams)
	}

	DecrementActiveStreams()
	streams = testutil.ToFloat64(ActiveStreams.WithLabelValues("test-pod"))
	if streams != 1 {
		t.Errorf("ActiveStreams after dec = %v, want 1", streams)
	}
}

func TestMultipleTenants(t *testing.T) {
	RequestsTotal.Reset()

	RecordRequest("tenant1", "openai", "gpt-4", "success", 1.0)
	RecordRequest("tenant2", "anthropic", "claude-3", "success", 2.0)
	RecordRequest("tenant1", "openai", "gpt-4", "error", 0.5)

	tenant1Success := testutil.ToFloat64(RequestsTotal.WithLabelValues("tenant1", "openai", "gpt-4", "success"))
	if tenant1Success != 1 {
		t.Errorf("tenant1 success = %v, want 1", tenant1Success)
	}

	tenant1Error := testutil.ToFloat64(RequestsTotal.WithLabelValues("tenant1", "openai", "gpt-4", "error"))
	if tenant1Error != 1 {
		t.Errorf("tenant1 error = %v, want 1", tenant1Error)
	}

	tenant2Success := testutil.ToFloat64(RequestsTotal.WithLabelValues("tenant2", "anthropic", "claude-3", "success"))
	if tenant2Success != 1 {
		t.Errorf("tenant2 success = %v, want 1", tenant2Success)
	}
}
