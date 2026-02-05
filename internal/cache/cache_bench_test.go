package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
)

func BenchmarkInMemoryCache_Set(b *testing.B) {
	c := NewInMemoryCache()
	ctx := context.Background()
	req := domain.ChatRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}
	key := GenerateCacheKey(req)
	resp := &domain.ChatResponse{
		ID:    "test-id",
		Model: "gpt-4",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, key, resp, 5*time.Minute)
	}
}

func BenchmarkInMemoryCache_Get_Hit(b *testing.B) {
	c := NewInMemoryCache()
	ctx := context.Background()
	req := domain.ChatRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}
	key := GenerateCacheKey(req)
	resp := &domain.ChatResponse{
		ID:    "test-id",
		Model: "gpt-4",
	}
	c.Set(ctx, key, resp, 5*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, key)
	}
}

func BenchmarkInMemoryCache_Get_Miss(b *testing.B) {
	c := NewInMemoryCache()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, "nonexistent-key")
	}
}

func BenchmarkInMemoryCache_Parallel(b *testing.B) {
	c := NewInMemoryCache()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100)
			resp := &domain.ChatResponse{
				ID:    fmt.Sprintf("id-%d", i),
				Model: "gpt-4",
			}

			if i%2 == 0 {
				c.Set(ctx, key, resp, 5*time.Minute)
			} else {
				c.Get(ctx, key)
			}
			i++
		}
	})
}

func BenchmarkGenerateCacheKey(b *testing.B) {
	req := domain.ChatRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
		Temperature: floatPtr(0.7),
		MaxTokens:   intPtr(1000),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateCacheKey(req)
	}
}

func floatPtr(f float64) *float64 { return &f }
func intPtr(i int) *int           { return &i }
