# Notifications Package

Event notifications with AWS SNS.

## Overview

Sends notifications for important system events like budget alerts and provider status changes.
Supports both AWS SNS (production) and in-memory (development).

## Notification Types

| Type | Description | Data |
|------|-------------|------|
| `budget_warning` | Budget threshold crossed (e.g., 50%) | tenant_id, usage_pct, budget_usd |
| `budget_critical` | Critical threshold crossed (e.g., 80%) | tenant_id, usage_pct, budget_usd |
| `budget_exceeded` | Budget fully consumed | tenant_id, usage_pct, budget_usd |
| `provider_down` | Provider health check failed | provider, error |
| `provider_up` | Provider recovered | provider |
| `rate_limited` | Tenant hit rate limit | tenant_id, limit_rpm |

## Interface

```go
type Notifier interface {
    Send(ctx context.Context, notification Notification) error
    Subscribe(ctx context.Context, topicArn, protocol, endpoint string) error
}

type Notification struct {
    Type     NotificationType       // Event type
    TenantID string                 // Affected tenant (if applicable)
    Message  string                 // Human-readable message
    Data     map[string]interface{} // Additional context
}
```

## Usage

### AWS SNS

```go
notifier, err := notifications.NewSNSNotifier(ctx, "us-east-1", topicArn)
if err != nil {
    log.Fatal(err)
}

err = notifier.Send(ctx, notifications.Notification{
    Type:     notifications.NotificationBudgetWarning,
    TenantID: "tenant-123",
    Message:  "Budget usage at 50%",
    Data: map[string]interface{}{
        "usage_pct":  50.0,
        "budget_usd": 100.0,
        "spent_usd":  50.0,
    },
})
```

### In-Memory (Testing)

```go
notifier := notifications.NewInMemoryNotifier()

// Register handler
notifier.OnNotification(func(n notifications.Notification) {
    fmt.Printf("Received: %s\n", n.Type)
})

// Send notification
notifier.Send(ctx, notification)

// Check sent notifications
all := notifier.GetNotifications()
```

## SNS Message Format

Messages are published as JSON:

```json
{
    "type": "budget_warning",
    "tenant_id": "tenant-123",
    "message": "Budget usage at 50%",
    "data": {
        "usage_pct": 50.0,
        "budget_usd": 100.0,
        "spent_usd": 50.0
    }
}
```

## Message Attributes

SNS messages include attributes for filtering:

| Attribute | Description |
|-----------|-------------|
| `Type` | Notification type (for subscription filters) |
| `TenantID` | Tenant identifier (if applicable) |

## Subscription Filtering

Create filtered subscriptions to route notifications:

```go
// Only budget notifications
notifier.Subscribe(ctx, topicArn, "email", "alerts@example.com")

// Use SNS filter policy for specific types:
// { "Type": ["budget_warning", "budget_critical", "budget_exceeded"] }
```

## AWS Configuration

Required IAM permissions:
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "sns:Publish",
                "sns:Subscribe"
            ],
            "Resource": "arn:aws:sns:*:*:ai-gateway-*"
        }
    ]
}
```

Environment:
```bash
export AWS_REGION=us-east-1
export SNS_TOPIC_ARN=arn:aws:sns:us-east-1:123456789:ai-gateway-alerts
```

## Integration with Budget Monitor

```go
budgetMonitor := budget.NewMonitor(costTracker, thresholds)

budgetMonitor.OnAlert(func(alert budget.Alert) {
    notifier.Send(ctx, notifications.Notification{
        Type:     notifications.NotificationType("budget_" + alert.Level),
        TenantID: alert.TenantID,
        Message:  alert.Message,
        Data: map[string]interface{}{
            "usage_pct":  alert.UsagePercent,
            "budget_usd": alert.BudgetUSD,
        },
    })
})
```

## Supported Protocols

SNS supports multiple delivery protocols:
- `email` - Email notifications
- `sms` - SMS messages
- `https` - Webhook endpoints
- `sqs` - SQS queue
- `lambda` - Lambda function

## Dependencies

- `github.com/aws/aws-sdk-go-v2/service/sns` - AWS SDK
