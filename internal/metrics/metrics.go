package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aigateway_requests_total",
			Help: "Total number of requests processed",
		},
		[]string{"tenant_id", "provider", "model", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aigateway_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"tenant_id", "provider", "model"},
	)

	TokensTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aigateway_tokens_total",
			Help: "Total number of tokens processed",
		},
		[]string{"tenant_id", "provider", "model", "type"},
	)

	CostTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aigateway_cost_usd_total",
			Help: "Total cost in USD",
		},
		[]string{"tenant_id", "provider", "model"},
	)

	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aigateway_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"tenant_id"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aigateway_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"tenant_id"},
	)

	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aigateway_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"provider"},
	)

	ProviderErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aigateway_provider_errors_total",
			Help: "Total number of provider errors",
		},
		[]string{"provider", "error_type"},
	)

	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aigateway_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"tenant_id"},
	)

	ActiveStreams = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aigateway_active_streams",
			Help: "Number of active streaming connections",
		},
		[]string{"pod"},
	)

	ActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aigateway_active_connections",
			Help: "Number of active HTTP connections being processed",
		},
		[]string{"pod"},
	)

	InstanceInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aigateway_instance_info",
			Help: "Instance information (always 1)",
		},
		[]string{"pod", "namespace", "version"},
	)

	BudgetUsageRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aigateway_budget_usage_ratio",
			Help: "Current budget usage ratio (0-1)",
		},
		[]string{"tenant_id"},
	)
)

func RecordRequest(tenantID, provider, model, status string, durationSec float64) {
	RequestsTotal.WithLabelValues(tenantID, provider, model, status).Inc()
	RequestDuration.WithLabelValues(tenantID, provider, model).Observe(durationSec)
}

func RecordTokens(tenantID, provider, model string, inputTokens, outputTokens int) {
	TokensTotal.WithLabelValues(tenantID, provider, model, "input").Add(float64(inputTokens))
	TokensTotal.WithLabelValues(tenantID, provider, model, "output").Add(float64(outputTokens))
}

func RecordCost(tenantID, provider, model string, costUSD float64) {
	CostTotal.WithLabelValues(tenantID, provider, model).Add(costUSD)
}

func RecordCacheHit(tenantID string) {
	CacheHits.WithLabelValues(tenantID).Inc()
}

func RecordCacheMiss(tenantID string) {
	CacheMisses.WithLabelValues(tenantID).Inc()
}

func RecordProviderError(provider, errorType string) {
	ProviderErrors.WithLabelValues(provider, errorType).Inc()
}

func RecordRateLimitHit(tenantID string) {
	RateLimitHits.WithLabelValues(tenantID).Inc()
}

func SetCircuitBreakerState(provider string, state int) {
	CircuitBreakerState.WithLabelValues(provider).Set(float64(state))
}

func SetBudgetUsage(tenantID string, ratio float64) {
	BudgetUsageRatio.WithLabelValues(tenantID).Set(ratio)
}

// Instance-aware metrics for horizontal scaling
var currentPodName string

// InitInstanceMetrics initializes instance-specific metrics.
// Should be called once at startup with pod identification.
func InitInstanceMetrics(podName, namespace, version string) {
	currentPodName = podName
	InstanceInfo.WithLabelValues(podName, namespace, version).Set(1)
}

// IncrementActiveConnections increments the active connection count for this pod.
func IncrementActiveConnections() {
	ActiveConnections.WithLabelValues(currentPodName).Inc()
}

// DecrementActiveConnections decrements the active connection count for this pod.
func DecrementActiveConnections() {
	ActiveConnections.WithLabelValues(currentPodName).Dec()
}

// IncrementActiveStreams increments the active stream count for this pod.
func IncrementActiveStreams() {
	ActiveStreams.WithLabelValues(currentPodName).Inc()
}

// DecrementActiveStreams decrements the active stream count for this pod.
func DecrementActiveStreams() {
	ActiveStreams.WithLabelValues(currentPodName).Dec()
}
