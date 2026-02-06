package budget

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/cost"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type AlertLevel string

const (
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
	AlertLevelExceeded AlertLevel = "exceeded"
)

type Alert struct {
	TenantID   string
	Level      AlertLevel
	Budget     float64
	CurrentUse float64
	Percentage float64
	Timestamp  time.Time
}

type AlertHandler func(alert Alert)

type Monitor struct {
	mu            sync.RWMutex
	tracker       cost.Tracker
	alertHandlers []AlertHandler
	thresholds    Thresholds
	deduplicator  AlertDeduplicator
}

type Thresholds struct {
	Warning  float64
	Critical float64
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		Warning:  0.8,
		Critical: 0.95,
	}
}

// MonitorOption configures a Monitor.
type MonitorOption func(*Monitor)

// WithDeduplicator sets a custom alert deduplicator.
// Use this to enable distributed deduplication with Redis.
func WithDeduplicator(d AlertDeduplicator) MonitorOption {
	return func(m *Monitor) {
		m.deduplicator = d
	}
}

// NewMonitor creates a new budget monitor.
// By default, it uses in-memory deduplication.
// Use WithDeduplicator option for distributed deduplication.
func NewMonitor(tracker cost.Tracker, thresholds Thresholds, opts ...MonitorOption) *Monitor {
	m := &Monitor{
		tracker:       tracker,
		thresholds:    thresholds,
		alertHandlers: make([]AlertHandler, 0),
		deduplicator:  NewInMemoryDeduplicator(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *Monitor) OnAlert(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandlers = append(m.alertHandlers, handler)
}

func (m *Monitor) Check(ctx context.Context, tenant *domain.Tenant) (*Alert, error) {
	if tenant.BudgetUSD <= 0 {
		return nil, nil
	}

	startOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)
	currentCost, err := m.tracker.GetTenantTotalCost(ctx, tenant.ID, startOfMonth)
	if err != nil {
		return nil, err
	}

	percentage := currentCost / tenant.BudgetUSD

	var level AlertLevel
	switch {
	case percentage >= 1.0:
		level = AlertLevelExceeded
	case percentage >= m.thresholds.Critical:
		level = AlertLevelCritical
	case percentage >= m.thresholds.Warning:
		level = AlertLevelWarning
	default:
		// Usage dropped below warning threshold, clear alert state
		m.deduplicator.ClearAlert(ctx, tenant.ID)
		return nil, nil
	}

	// Check if we should send this alert (deduplication)
	if !m.deduplicator.ShouldAlert(ctx, tenant.ID, level) {
		return nil, nil
	}

	alert := &Alert{
		TenantID:   tenant.ID,
		Level:      level,
		Budget:     tenant.BudgetUSD,
		CurrentUse: currentCost,
		Percentage: percentage * 100,
		Timestamp:  time.Now(),
	}

	m.mu.RLock()
	handlers := make([]AlertHandler, len(m.alertHandlers))
	copy(handlers, m.alertHandlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		handler(*alert)
	}

	return alert, nil
}

func (m *Monitor) IsBudgetExceeded(ctx context.Context, tenant *domain.Tenant) (bool, error) {
	if tenant.BudgetUSD <= 0 {
		return false, nil
	}

	startOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)
	currentCost, err := m.tracker.GetTenantTotalCost(ctx, tenant.ID, startOfMonth)
	if err != nil {
		return false, err
	}

	return currentCost >= tenant.BudgetUSD, nil
}

func LogAlertHandler(alert Alert) {
	slog.Warn("budget alert",
		"tenant_id", alert.TenantID,
		"level", alert.Level,
		"budget", alert.Budget,
		"current_use", alert.CurrentUse,
		"percentage", alert.Percentage,
	)
}
