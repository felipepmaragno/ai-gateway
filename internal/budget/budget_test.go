package budget

import (
	"context"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/cost"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type mockTracker struct {
	costs map[string]float64
}

func newMockTracker() *mockTracker {
	return &mockTracker{costs: make(map[string]float64)}
}

func (m *mockTracker) Record(ctx context.Context, record cost.UsageRecord) error {
	m.costs[record.TenantID] += record.CostUSD
	return nil
}

func (m *mockTracker) GetTenantTotalCost(ctx context.Context, tenantID string, since time.Time) (float64, error) {
	return m.costs[tenantID], nil
}

func (m *mockTracker) GetTenantUsage(ctx context.Context, tenantID string, since time.Time) ([]cost.UsageRecord, error) {
	return nil, nil
}

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()

	if th.Warning != 0.8 {
		t.Errorf("Warning threshold = %v, want 0.8", th.Warning)
	}
	if th.Critical != 0.95 {
		t.Errorf("Critical threshold = %v, want 0.95", th.Critical)
	}
}

func TestNewMonitor(t *testing.T) {
	tracker := newMockTracker()
	th := DefaultThresholds()

	monitor := NewMonitor(tracker, th)

	if monitor == nil {
		t.Fatal("NewMonitor() returned nil")
	}
	if monitor.tracker != tracker {
		t.Error("tracker not set correctly")
	}
}

func TestMonitor_Check_NoBudget(t *testing.T) {
	tracker := newMockTracker()
	monitor := NewMonitor(tracker, DefaultThresholds())

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 0, // No budget set
	}

	alert, err := monitor.Check(context.Background(), tenant)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if alert != nil {
		t.Error("Check() should return nil alert for tenant without budget")
	}
}

func TestMonitor_Check_UnderBudget(t *testing.T) {
	tracker := newMockTracker()
	tracker.costs["tenant1"] = 50.0 // 50% of budget

	monitor := NewMonitor(tracker, DefaultThresholds())

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 100.0,
	}

	alert, err := monitor.Check(context.Background(), tenant)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if alert != nil {
		t.Error("Check() should return nil alert when under warning threshold")
	}
}

func TestMonitor_Check_WarningLevel(t *testing.T) {
	tracker := newMockTracker()
	tracker.costs["tenant1"] = 85.0 // 85% of budget

	monitor := NewMonitor(tracker, DefaultThresholds())

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 100.0,
	}

	alert, err := monitor.Check(context.Background(), tenant)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if alert == nil {
		t.Fatal("Check() should return alert at warning level")
	}
	if alert.Level != AlertLevelWarning {
		t.Errorf("alert.Level = %v, want %v", alert.Level, AlertLevelWarning)
	}
	if alert.TenantID != "tenant1" {
		t.Errorf("alert.TenantID = %v, want tenant1", alert.TenantID)
	}
}

func TestMonitor_Check_CriticalLevel(t *testing.T) {
	tracker := newMockTracker()
	tracker.costs["tenant1"] = 96.0 // 96% of budget

	monitor := NewMonitor(tracker, DefaultThresholds())

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 100.0,
	}

	alert, err := monitor.Check(context.Background(), tenant)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if alert == nil {
		t.Fatal("Check() should return alert at critical level")
	}
	if alert.Level != AlertLevelCritical {
		t.Errorf("alert.Level = %v, want %v", alert.Level, AlertLevelCritical)
	}
}

func TestMonitor_Check_ExceededLevel(t *testing.T) {
	tracker := newMockTracker()
	tracker.costs["tenant1"] = 110.0 // 110% of budget

	monitor := NewMonitor(tracker, DefaultThresholds())

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 100.0,
	}

	alert, err := monitor.Check(context.Background(), tenant)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if alert == nil {
		t.Fatal("Check() should return alert when exceeded")
	}
	if alert.Level != AlertLevelExceeded {
		t.Errorf("alert.Level = %v, want %v", alert.Level, AlertLevelExceeded)
	}
}

func TestMonitor_Check_NoRepeatAlerts(t *testing.T) {
	tracker := newMockTracker()
	tracker.costs["tenant1"] = 85.0

	monitor := NewMonitor(tracker, DefaultThresholds())

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 100.0,
	}

	// First check should return alert
	alert1, _ := monitor.Check(context.Background(), tenant)
	if alert1 == nil {
		t.Fatal("First check should return alert")
	}

	// Second check at same level should not return alert
	alert2, _ := monitor.Check(context.Background(), tenant)
	if alert2 != nil {
		t.Error("Second check at same level should not return alert")
	}
}

func TestMonitor_OnAlert(t *testing.T) {
	tracker := newMockTracker()
	tracker.costs["tenant1"] = 85.0

	monitor := NewMonitor(tracker, DefaultThresholds())

	var receivedAlert *Alert
	monitor.OnAlert(func(a Alert) {
		receivedAlert = &a
	})

	tenant := &domain.Tenant{
		ID:        "tenant1",
		BudgetUSD: 100.0,
	}

	monitor.Check(context.Background(), tenant)

	if receivedAlert == nil {
		t.Fatal("Alert handler should have been called")
	}
	if receivedAlert.TenantID != "tenant1" {
		t.Errorf("receivedAlert.TenantID = %v, want tenant1", receivedAlert.TenantID)
	}
}

func TestMonitor_IsBudgetExceeded(t *testing.T) {
	tracker := newMockTracker()
	monitor := NewMonitor(tracker, DefaultThresholds())

	tests := []struct {
		name       string
		budget     float64
		cost       float64
		wantExceed bool
	}{
		{"no budget", 0, 100, false},
		{"under budget", 100, 50, false},
		{"at budget", 100, 100, true},
		{"over budget", 100, 150, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker.costs["tenant1"] = tt.cost

			tenant := &domain.Tenant{
				ID:        "tenant1",
				BudgetUSD: tt.budget,
			}

			exceeded, err := monitor.IsBudgetExceeded(context.Background(), tenant)
			if err != nil {
				t.Fatalf("IsBudgetExceeded() error = %v", err)
			}
			if exceeded != tt.wantExceed {
				t.Errorf("IsBudgetExceeded() = %v, want %v", exceeded, tt.wantExceed)
			}
		})
	}
}

func TestLogAlertHandler(t *testing.T) {
	// Just verify it doesn't panic
	alert := Alert{
		TenantID:   "tenant1",
		Level:      AlertLevelWarning,
		Budget:     100.0,
		CurrentUse: 85.0,
		Percentage: 85.0,
		Timestamp:  time.Now(),
	}

	LogAlertHandler(alert)
}
