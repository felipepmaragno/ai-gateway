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

func TestGenerateCacheKey_IncludesModel(t *testing.T) {
	req1 := domain.ChatRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	req2 := domain.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)

	if key1 == key2 {
		t.Error("different models should produce different keys")
	}
}

func TestGenerateCacheKey_IncludesTemperature(t *testing.T) {
	temp1 := 0.0
	temp2 := 0.5

	req1 := domain.ChatRequest{
		Model:       "gpt-4",
		Messages:    []domain.Message{{Role: "user", Content: "Hello"}},
		Temperature: &temp1,
	}

	req2 := domain.ChatRequest{
		Model:       "gpt-4",
		Messages:    []domain.Message{{Role: "user", Content: "Hello"}},
		Temperature: &temp2,
	}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)

	if key1 == key2 {
		t.Error("different temperatures should produce different keys")
	}
}

func TestGenerateCacheKey_IncludesMaxTokens(t *testing.T) {
	max1 := 100
	max2 := 200

	req1 := domain.ChatRequest{
		Model:     "gpt-4",
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
		MaxTokens: &max1,
	}

	req2 := domain.ChatRequest{
		Model:     "gpt-4",
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
		MaxTokens: &max2,
	}

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)

	if key1 == key2 {
		t.Error("different max_tokens should produce different keys")
	}
}

func TestGenerateCacheKey_HasPrefix(t *testing.T) {
	req := domain.ChatRequest{
		Model:    "gpt-4",
		Messages: []domain.Message{{Role: "user", Content: "Hello"}},
	}

	key := GenerateCacheKey(req)

	if len(key) < 6 || key[:6] != "cache:" {
		t.Errorf("key should have 'cache:' prefix, got %s", key)
	}
}

func TestInMemoryCache_Overwrite(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	resp1 := &domain.ChatResponse{ID: "first"}
	resp2 := &domain.ChatResponse{ID: "second"}

	c.Set(ctx, "key", resp1, time.Minute)
	c.Set(ctx, "key", resp2, time.Minute)

	cached, ok := c.Get(ctx, "key")
	if !ok {
		t.Fatal("expected cache hit")
	}

	if cached.ID != "second" {
		t.Errorf("expected overwritten value, got %s", cached.ID)
	}
}

func TestInMemoryCache_MultipleKeys(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		resp := &domain.ChatResponse{ID: string(rune('a' + i%26))}
		key := string(rune('a' + i%26))
		c.Set(ctx, key, resp, time.Minute)
	}

	// Should be able to retrieve all 26 unique keys
	for i := 0; i < 26; i++ {
		key := string(rune('a' + i))
		_, ok := c.Get(ctx, key)
		if !ok {
			t.Errorf("expected cache hit for key %s", key)
		}
	}
}

func TestInMemoryCache_ConcurrentAccess(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 1000; i++ {
			resp := &domain.ChatResponse{ID: "test"}
			c.Set(ctx, "concurrent-key", resp, time.Minute)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 1000; i++ {
			c.Get(ctx, "concurrent-key")
		}
		done <- true
	}()

	<-done
	<-done
}
