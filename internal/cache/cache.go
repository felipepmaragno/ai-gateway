// Package cache provides response caching for deterministic LLM requests.
// It supports both in-memory (single instance) and Redis (distributed) backends.
// Caching reduces latency and costs by returning stored responses for identical requests.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/redis/go-redis/v9"
)

// Cache defines the interface for response caching backends.
type Cache interface {
	Get(ctx context.Context, key string) (*domain.ChatResponse, bool)
	Set(ctx context.Context, key string, resp *domain.ChatResponse, ttl time.Duration) error
}

// GenerateCacheKey creates a unique cache key from a chat request.
// The key is a SHA-256 hash of the model, messages, temperature, and max_tokens.
func GenerateCacheKey(req domain.ChatRequest) string {
	data, _ := json.Marshal(struct {
		Model       string           `json:"model"`
		Messages    []domain.Message `json:"messages"`
		Temperature *float64         `json:"temperature,omitempty"`
		MaxTokens   *int             `json:"max_tokens,omitempty"`
	}{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})

	hash := sha256.Sum256(data)
	return "cache:" + hex.EncodeToString(hash[:])
}

type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	response  *domain.ChatResponse
	expiresAt time.Time
}

func NewInMemoryCache() *InMemoryCache {
	c := &InMemoryCache{
		items: make(map[string]*cacheItem),
	}
	go c.cleanup()
	return c
}

func (c *InMemoryCache) Get(ctx context.Context, key string) (*domain.ChatResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.response, true
}

func (c *InMemoryCache) Set(ctx context.Context, key string, resp *domain.ChatResponse, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheItem{
		response:  resp,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

func (c *InMemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) (*domain.ChatResponse, bool) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var resp domain.ChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false
	}

	return &resp, true
}

func (c *RedisCache) Set(ctx context.Context, key string, resp *domain.ChatResponse, ttl time.Duration) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}
