package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type NotificationType string

const (
	NotificationBudgetWarning  NotificationType = "budget_warning"
	NotificationBudgetCritical NotificationType = "budget_critical"
	NotificationBudgetExceeded NotificationType = "budget_exceeded"
	NotificationProviderDown   NotificationType = "provider_down"
	NotificationProviderUp     NotificationType = "provider_up"
	NotificationRateLimited    NotificationType = "rate_limited"
)

type Notification struct {
	Type     NotificationType       `json:"type"`
	TenantID string                 `json:"tenant_id,omitempty"`
	Message  string                 `json:"message"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

type Notifier interface {
	Send(ctx context.Context, notification Notification) error
	Subscribe(ctx context.Context, topicArn, protocol, endpoint string) error
}

type SNSNotifier struct {
	client   *sns.Client
	topicArn string
}

func NewSNSNotifier(ctx context.Context, region, topicArn string) (*SNSNotifier, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return &SNSNotifier{
		client:   sns.NewFromConfig(cfg),
		topicArn: topicArn,
	}, nil
}

func NewSNSNotifierWithConfig(cfg aws.Config, topicArn string) *SNSNotifier {
	return &SNSNotifier{
		client:   sns.NewFromConfig(cfg),
		topicArn: topicArn,
	}
}

func (n *SNSNotifier) Send(ctx context.Context, notification Notification) error {
	message, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	input := &sns.PublishInput{
		TopicArn: aws.String(n.topicArn),
		Message:  aws.String(string(message)),
		MessageAttributes: map[string]snstypes.MessageAttributeValue{
			"Type": {
				DataType:    aws.String("String"),
				StringValue: aws.String(string(notification.Type)),
			},
		},
	}

	if notification.TenantID != "" {
		input.MessageAttributes["TenantID"] = snstypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(notification.TenantID),
		}
	}

	_, err = n.client.Publish(ctx, input)
	if err != nil {
		return fmt.Errorf("publish notification: %w", err)
	}

	slog.Info("notification sent",
		"type", notification.Type,
		"tenant_id", notification.TenantID,
	)

	return nil
}

func (n *SNSNotifier) Subscribe(ctx context.Context, topicArn, protocol, endpoint string) error {
	input := &sns.SubscribeInput{
		TopicArn: aws.String(topicArn),
		Protocol: aws.String(protocol),
		Endpoint: aws.String(endpoint),
	}

	_, err := n.client.Subscribe(ctx, input)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	return nil
}

type InMemoryNotifier struct {
	mu            sync.Mutex
	notifications []Notification
	handlers      []func(Notification)
}

func NewInMemoryNotifier() *InMemoryNotifier {
	return &InMemoryNotifier{
		notifications: make([]Notification, 0),
		handlers:      make([]func(Notification), 0),
	}
}

func (n *InMemoryNotifier) Send(ctx context.Context, notification Notification) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.notifications = append(n.notifications, notification)

	for _, handler := range n.handlers {
		handler(notification)
	}

	slog.Info("notification sent (in-memory)",
		"type", notification.Type,
		"tenant_id", notification.TenantID,
	)

	return nil
}

func (n *InMemoryNotifier) Subscribe(ctx context.Context, topicArn, protocol, endpoint string) error {
	return nil
}

func (n *InMemoryNotifier) OnNotification(handler func(Notification)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.handlers = append(n.handlers, handler)
}

func (n *InMemoryNotifier) GetNotifications() []Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]Notification, len(n.notifications))
	copy(result, n.notifications)
	return result
}

func (n *InMemoryNotifier) Clear() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifications = make([]Notification, 0)
}
