package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type Provider struct {
	client *bedrockruntime.Client
	region string
}

func New(ctx context.Context, region string) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := bedrockruntime.NewFromConfig(cfg)

	return &Provider{
		client: client,
		region: region,
	}, nil
}

func NewWithConfig(cfg aws.Config) *Provider {
	return &Provider{
		client: bedrockruntime.NewFromConfig(cfg),
		region: cfg.Region,
	}
}

func (p *Provider) ID() string {
	return "bedrock"
}

func (p *Provider) ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	bedrockReq := toBedrockRequest(req)

	body, err := json.Marshal(bedrockReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	modelID := mapModelID(req.Model)

	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        body,
	}

	output, err := p.client.InvokeModel(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("invoke model: %w", err)
	}

	return parseBedrockResponse(output.Body, req.Model)
}

func (p *Provider) ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	chunks := make(chan domain.StreamChunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		bedrockReq := toBedrockRequest(req)
		body, err := json.Marshal(bedrockReq)
		if err != nil {
			errs <- fmt.Errorf("marshal request: %w", err)
			return
		}

		modelID := mapModelID(req.Model)

		input := &bedrockruntime.InvokeModelWithResponseStreamInput{
			ModelId:     aws.String(modelID),
			ContentType: aws.String("application/json"),
			Accept:      aws.String("application/json"),
			Body:        body,
		}

		output, err := p.client.InvokeModelWithResponseStream(ctx, input)
		if err != nil {
			errs <- fmt.Errorf("invoke model stream: %w", err)
			return
		}

		stream := output.GetStream()
		defer stream.Close()

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				var chunkResp bedrockStreamChunk
				if err := json.Unmarshal(v.Value.Bytes, &chunkResp); err != nil {
					continue
				}

				if chunkResp.Type == "content_block_delta" && chunkResp.Delta != nil {
					chunk := domain.StreamChunk{
						ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   req.Model,
						Choices: []domain.Choice{
							{
								Index: 0,
								Delta: &domain.Delta{
									Content: chunkResp.Delta.Text,
								},
							},
						},
					}

					select {
					case chunks <- chunk:
					case <-ctx.Done():
						return
					}
				}

				if chunkResp.Type == "message_stop" {
					return
				}
			}
		}

		if err := stream.Err(); err != nil {
			errs <- fmt.Errorf("stream error: %w", err)
		}
	}()

	return chunks, errs
}

func (p *Provider) Models(ctx context.Context) ([]domain.Model, error) {
	models := []domain.Model{
		{ID: "anthropic.claude-3-5-sonnet-20241022-v2:0", Object: "model", OwnedBy: "anthropic", Provider: "bedrock"},
		{ID: "anthropic.claude-3-5-haiku-20241022-v1:0", Object: "model", OwnedBy: "anthropic", Provider: "bedrock"},
		{ID: "anthropic.claude-3-opus-20240229-v1:0", Object: "model", OwnedBy: "anthropic", Provider: "bedrock"},
		{ID: "anthropic.claude-3-sonnet-20240229-v1:0", Object: "model", OwnedBy: "anthropic", Provider: "bedrock"},
		{ID: "anthropic.claude-3-haiku-20240307-v1:0", Object: "model", OwnedBy: "anthropic", Provider: "bedrock"},
		{ID: "amazon.titan-text-express-v1", Object: "model", OwnedBy: "amazon", Provider: "bedrock"},
		{ID: "amazon.titan-text-lite-v1", Object: "model", OwnedBy: "amazon", Provider: "bedrock"},
		{ID: "meta.llama3-70b-instruct-v1:0", Object: "model", OwnedBy: "meta", Provider: "bedrock"},
		{ID: "meta.llama3-8b-instruct-v1:0", Object: "model", OwnedBy: "meta", Provider: "bedrock"},
	}
	return models, nil
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	return nil
}

type bedrockRequest struct {
	AnthropicVersion string           `json:"anthropic_version,omitempty"`
	MaxTokens        int              `json:"max_tokens"`
	Messages         []bedrockMessage `json:"messages"`
	System           string           `json:"system,omitempty"`
}

type bedrockMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type bedrockResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []contentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      bedrockUsage   `json:"usage"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type bedrockUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type bedrockStreamChunk struct {
	Type  string       `json:"type"`
	Index int          `json:"index,omitempty"`
	Delta *streamDelta `json:"delta,omitempty"`
}

type streamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func mapModelID(model string) string {
	modelMap := map[string]string{
		"claude-3-5-sonnet": "anthropic.claude-3-5-sonnet-20241022-v2:0",
		"claude-3-5-haiku":  "anthropic.claude-3-5-haiku-20241022-v1:0",
		"claude-3-opus":     "anthropic.claude-3-opus-20240229-v1:0",
		"claude-3-sonnet":   "anthropic.claude-3-sonnet-20240229-v1:0",
		"claude-3-haiku":    "anthropic.claude-3-haiku-20240307-v1:0",
		"titan-text":        "amazon.titan-text-express-v1",
		"llama3-70b":        "meta.llama3-70b-instruct-v1:0",
		"llama3-8b":         "meta.llama3-8b-instruct-v1:0",
	}

	if mapped, ok := modelMap[model]; ok {
		return mapped
	}
	return model
}

func toBedrockRequest(req domain.ChatRequest) bedrockRequest {
	var systemPrompt string
	var messages []bedrockMessage

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}
		messages = append(messages, bedrockMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	return bedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        maxTokens,
		Messages:         messages,
		System:           systemPrompt,
	}
}

func parseBedrockResponse(body []byte, model string) (*domain.ChatResponse, error) {
	var resp bedrockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &domain.ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []domain.Choice{
			{
				Index: 0,
				Message: &domain.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: mapStopReason(resp.StopReason),
			},
		},
		Usage: domain.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

func mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return reason
	}
}
