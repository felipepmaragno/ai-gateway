# Queue Package

Asynchronous request processing with AWS SQS.

## Overview

Enables async processing of LLM requests for long-running operations.
Clients submit requests to a queue and poll for responses, avoiding HTTP timeouts.

## Architecture

```
Client                    Gateway                      Worker
  │                          │                           │
  │  POST /v1/async/chat     │                           │
  │ ───────────────────────► │                           │
  │                          │  SendRequest()            │
  │                          │ ─────────────────────────►│
  │  { request_id: "..." }   │                           │
  │ ◄─────────────────────── │                           │
  │                          │                           │
  │                          │         ReceiveRequests() │
  │                          │ ◄─────────────────────────│
  │                          │                           │
  │                          │         Process request   │
  │                          │                           │
  │                          │         SendResponse()    │
  │                          │ ◄─────────────────────────│
  │                          │                           │
  │  GET /v1/async/{id}      │                           │
  │ ───────────────────────► │                           │
  │  { response: {...} }     │                           │
  │ ◄─────────────────────── │                           │
```

## Message Types

### AsyncRequest

```go
type AsyncRequest struct {
    ID        string             // Unique request identifier
    TenantID  string             // Tenant making the request
    Request   domain.ChatRequest // The actual chat request
    Provider  string             // Optional: preferred provider
    Callback  string             // Optional: webhook URL for completion
    CreatedAt time.Time          // Request timestamp
}
```

### AsyncResponse

```go
type AsyncResponse struct {
    RequestID string               // Matches AsyncRequest.ID
    TenantID  string               // Tenant identifier
    Response  *domain.ChatResponse // The chat response (if successful)
    Error     string               // Error message (if failed)
    CreatedAt time.Time            // Response timestamp
}
```

## Backends

| Backend | Use Case | Persistence |
|---------|----------|-------------|
| SQS | Production, distributed | Yes |
| In-Memory | Development, testing | No |

## SQS Configuration

Required environment variables:
```
AWS_REGION=us-east-1
SQS_REQUEST_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789/ai-gateway-requests
SQS_RESPONSE_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789/ai-gateway-responses
```

## Usage

### Producer (API Gateway)

```go
queue, err := queue.NewSQSQueue(ctx, region, requestURL, responseURL)

req := queue.AsyncRequest{
    ID:        uuid.New().String(),
    TenantID:  tenant.ID,
    Request:   chatRequest,
    CreatedAt: time.Now(),
}

err = queue.SendRequest(ctx, req)
```

### Consumer (Worker)

```go
for {
    requests, err := queue.ReceiveRequests(ctx, 10)
    if err != nil {
        continue
    }

    for _, req := range requests {
        resp, err := processRequest(ctx, req)
        
        asyncResp := queue.AsyncResponse{
            RequestID: req.ID,
            TenantID:  req.TenantID,
            Response:  resp,
            CreatedAt: time.Now(),
        }
        if err != nil {
            asyncResp.Error = err.Error()
        }

        queue.SendResponse(ctx, asyncResp)
        queue.DeleteRequest(ctx, receiptHandle)
    }
}
```

## SQS Settings

Recommended queue configuration:
- **Visibility Timeout**: 5 minutes (longer than max LLM response time)
- **Message Retention**: 4 days
- **Long Polling**: 20 seconds (reduces API calls)
- **Dead Letter Queue**: After 3 failed attempts

## Message Attributes

Messages include SQS attributes for filtering:
- `TenantID` - For tenant-specific processing
- `RequestID` - For correlation

## Dependencies

- `github.com/aws/aws-sdk-go-v2/service/sqs` - AWS SQS client
- `internal/domain` - Request/response types
