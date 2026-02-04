package cache

import (
	"context"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func TestInMemoryCache_SetAndGet(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	resp := &domain.ChatResponse{
		ID:     "test-id",
		Object: "chat.completion",
		Model:  "test-model",
	}

	err := c.Set(ctx, "key1", resp, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cached, ok := c.Get(ctx, "key1")
	if !ok {
		t.Fatal("expected cache hit")
	}

	if cached.ID != resp.ID {
		t.Errorf("expected ID %s, got %s", resp.ID, cached.ID)
	}
}

func TestInMemoryCache_Miss(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	_, ok := c.Get(ctx, "nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestInMemoryCache_Expiration(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	resp := &domain.ChatResponse{ID: "test-id"}

	err := c.Set(ctx, "key1", resp, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := c.Get(ctx, "key1")
	if !ok {
		t.Fatal("expected cache hit before expiration")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = c.Get(ctx, "key1")
	if ok {
		t.Error("expected cache miss after expiration")
	}
}

func TestGenerateCacheKey_Deterministic(t *testing.T) {
	req := domain.ChatRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	key1 := GenerateCacheKey(req)
	key2 := GenerateCacheKey(req)

	if key1 != key2 {
		t.Error("expected same key for same request")
	}
}

func TestGenerateCacheKey_DifferentForDifferentRequests(t *testing.T) {
	req1 := domain.ChatRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	req2 := domain.ChatRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)

	if key1 == key2 {
		t.Error("expected different keys for different requests")
	}
}
