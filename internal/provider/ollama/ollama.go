package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

type Provider struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string) *Provider {
	return &Provider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *Provider) ID() string {
	return "ollama"
}

func (p *Provider) ChatCompletion(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	ollamaReq := toOllamaRequest(req)

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return toOpenAIResponse(ollamaResp, req.Model), nil
}

func (p *Provider) ChatCompletionStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	chunks := make(chan domain.StreamChunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		ollamaReq := toOllamaRequest(req)
		ollamaReq.Stream = true

		body, err := json.Marshal(ollamaReq)
		if err != nil {
			errs <- fmt.Errorf("marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
		if err != nil {
			errs <- fmt.Errorf("create request: %w", err)
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(httpReq)
		if err != nil {
			errs <- fmt.Errorf("do request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errs <- fmt.Errorf("ollama error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var ollamaChunk ollamaStreamChunk
			if err := json.Unmarshal([]byte(line), &ollamaChunk); err != nil {
				continue
			}

			chunk := toOpenAIStreamChunk(ollamaChunk, req.Model)

			select {
			case chunks <- chunk:
			case <-ctx.Done():
				return
			}

			if ollamaChunk.Done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- fmt.Errorf("scan error: %w", err)
		}
	}()

	return chunks, errs
}

func (p *Provider) Models(ctx context.Context) ([]domain.Model, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error: status=%d", resp.StatusCode)
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]domain.Model, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = domain.Model{
			ID:       m.Name,
			Object:   "model",
			OwnedBy:  "ollama",
			Provider: "ollama",
		}
	}

	return models, nil
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", http.NoBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama unhealthy: status=%d", resp.StatusCode)
	}

	return nil
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64  `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type ollamaChatResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

type ollamaStreamChunk struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

func toOllamaRequest(req domain.ChatRequest) ollamaChatRequest {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	ollamaReq := ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream,
	}

	if req.Temperature != nil || req.MaxTokens != nil || req.TopP != nil || len(req.Stop) > 0 {
		ollamaReq.Options = &ollamaOptions{}
		if req.Temperature != nil {
			ollamaReq.Options.Temperature = *req.Temperature
		}
		if req.MaxTokens != nil {
			ollamaReq.Options.NumPredict = *req.MaxTokens
		}
		if req.TopP != nil {
			ollamaReq.Options.TopP = *req.TopP
		}
		if len(req.Stop) > 0 {
			ollamaReq.Options.Stop = req.Stop
		}
	}

	return ollamaReq
}

func toOpenAIResponse(resp ollamaChatResponse, model string) *domain.ChatResponse {
	return &domain.ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []domain.Choice{
			{
				Index: 0,
				Message: &domain.Message{
					Role:    resp.Message.Role,
					Content: resp.Message.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: domain.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}
}

func toOpenAIStreamChunk(chunk ollamaStreamChunk, model string) domain.StreamChunk {
	finishReason := ""
	if chunk.Done {
		finishReason = "stop"
	}

	return domain.StreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []domain.Choice{
			{
				Index: 0,
				Delta: &domain.Delta{
					Content: chunk.Message.Content,
				},
				FinishReason: finishReason,
			},
		},
	}
}
