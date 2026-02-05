# Budget Package

Budget monitoring and alerting for tenant spending.

## Overview

Monitors tenant spending against configured budgets and triggers alerts
at configurable thresholds (e.g., 50%, 80%, 100% of budget).

## Components

### Monitor

```go
monitor := budget.NewMonitor(costTracker, budget.DefaultThresholds())
monitor.OnAlert(budget.LogAlertHandler)
```

### Thresholds

Default thresholds:
- 50% - Warning
- 80% - Critical
- 100% - Budget exceeded

Custom thresholds:
```go
thresholds := []budget.Threshold{
    {Percentage: 0.5, Level: "warning"},
    {Percentage: 0.8, Level: "critical"},
    {Percentage: 1.0, Level: "exceeded"},
}
```

### Alert Handlers

Alerts can be sent to multiple destinations:

```go
// Log alerts
monitor.OnAlert(budget.LogAlertHandler)

// SNS notifications (AWS)
monitor.OnAlert(snsNotifier.SendAlert)

// Custom handler
monitor.OnAlert(func(alert budget.Alert) {
    // Send to Slack, email, etc.
})
```

## Interface

```go
type Monitor interface {
    Check(ctx context.Context, tenant *domain.Tenant) (float64, error)
    IsBudgetExceeded(ctx context.Context, tenant *domain.Tenant) (bool, error)
    OnAlert(handler AlertHandler)
}
```

## Usage Flow

1. After each request, handler calls `monitor.Check()`
2. Monitor calculates current month's spending
3. If threshold crossed, triggers alert handlers
4. If budget exceeded, subsequent requests return 402 Payment Required

## Dependencies

- `internal/cost` - Usage tracking
- `internal/domain` - Tenant types
- `internal/notifications` - SNS alerts (optional)
