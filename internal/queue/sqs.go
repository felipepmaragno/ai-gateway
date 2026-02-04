package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type AsyncRequest struct {
	ID        string             `json:"id"`
	TenantID  string             `json:"tenant_id"`
	Request   domain.ChatRequest `json:"request"`
	Provider  string             `json:"provider,omitempty"`
	Callback  string             `json:"callback,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

type AsyncResponse struct {
	RequestID string               `json:"request_id"`
	TenantID  string               `json:"tenant_id"`
	Response  *domain.ChatResponse `json:"response,omitempty"`
	Error     string               `json:"error,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
}

type Queue interface {
	SendRequest(ctx context.Context, req AsyncRequest) error
	ReceiveRequests(ctx context.Context, maxMessages int) ([]AsyncRequest, error)
	DeleteRequest(ctx context.Context, receiptHandle string) error
	SendResponse(ctx context.Context, resp AsyncResponse) error
}

type SQSQueue struct {
	client           *sqs.Client
	requestQueueURL  string
	responseQueueURL string
}

func NewSQSQueue(ctx context.Context, region, requestQueueURL, responseQueueURL string) (*SQSQueue, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return &SQSQueue{
		client:           sqs.NewFromConfig(cfg),
		requestQueueURL:  requestQueueURL,
		responseQueueURL: responseQueueURL,
	}, nil
}

func NewSQSQueueWithConfig(cfg aws.Config, requestQueueURL, responseQueueURL string) *SQSQueue {
	return &SQSQueue{
		client:           sqs.NewFromConfig(cfg),
		requestQueueURL:  requestQueueURL,
		responseQueueURL: responseQueueURL,
	}
}

func (q *SQSQueue) SendRequest(ctx context.Context, req AsyncRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.requestQueueURL),
		MessageBody: aws.String(string(body)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"TenantID": {
				DataType:    aws.String("String"),
				StringValue: aws.String(req.TenantID),
			},
			"RequestID": {
				DataType:    aws.String("String"),
				StringValue: aws.String(req.ID),
			},
		},
	}

	_, err = q.client.SendMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

func (q *SQSQueue) ReceiveRequests(ctx context.Context, maxMessages int) ([]AsyncRequest, error) {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(q.requestQueueURL),
		MaxNumberOfMessages:   int32(maxMessages),
		WaitTimeSeconds:       20,
		MessageAttributeNames: []string{"All"},
	}

	result, err := q.client.ReceiveMessage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("receive messages: %w", err)
	}

	requests := make([]AsyncRequest, 0, len(result.Messages))
	for _, msg := range result.Messages {
		var req AsyncRequest
		if err := json.Unmarshal([]byte(*msg.Body), &req); err != nil {
			slog.Warn("failed to unmarshal message", "error", err)
			continue
		}
		requests = append(requests, req)
	}

	return requests, nil
}

func (q *SQSQueue) DeleteRequest(ctx context.Context, receiptHandle string) error {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.requestQueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	}

	_, err := q.client.DeleteMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}

	return nil
}

func (q *SQSQueue) SendResponse(ctx context.Context, resp AsyncResponse) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.responseQueueURL),
		MessageBody: aws.String(string(body)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"TenantID": {
				DataType:    aws.String("String"),
				StringValue: aws.String(resp.TenantID),
			},
			"RequestID": {
				DataType:    aws.String("String"),
				StringValue: aws.String(resp.RequestID),
			},
		},
	}

	_, err = q.client.SendMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("send response: %w", err)
	}

	return nil
}

type InMemoryQueue struct {
	mu        sync.Mutex
	requests  []AsyncRequest
	responses []AsyncResponse
}

func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{
		requests:  make([]AsyncRequest, 0),
		responses: make([]AsyncResponse, 0),
	}
}

func (q *InMemoryQueue) SendRequest(ctx context.Context, req AsyncRequest) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.requests = append(q.requests, req)
	return nil
}

func (q *InMemoryQueue) ReceiveRequests(ctx context.Context, maxMessages int) ([]AsyncRequest, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := maxMessages
	if count > len(q.requests) {
		count = len(q.requests)
	}

	result := make([]AsyncRequest, count)
	copy(result, q.requests[:count])
	q.requests = q.requests[count:]

	return result, nil
}

func (q *InMemoryQueue) DeleteRequest(ctx context.Context, receiptHandle string) error {
	return nil
}

func (q *InMemoryQueue) SendResponse(ctx context.Context, resp AsyncResponse) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.responses = append(q.responses, resp)
	return nil
}

func (q *InMemoryQueue) GetResponses() []AsyncResponse {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]AsyncResponse, len(q.responses))
	copy(result, q.responses)
	return result
}
