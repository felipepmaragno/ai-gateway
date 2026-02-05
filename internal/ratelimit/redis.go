package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRateLimiter struct {
	client *redis.Client
}

func NewRedisRateLimiter(redisURL string) (*RedisRateLimiter, error) {
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

	return &RedisRateLimiter{client: client}, nil
}

func (r *RedisRateLimiter) Allow(ctx context.Context, tenantID string, limit int) (bool, int, time.Time, error) {
	key := "ratelimit:" + tenantID
	now := time.Now()
	windowStart := now.Add(-time.Minute)
	windowEnd := now.Add(time.Minute)

	pipe := r.client.Pipeline()

	pipe.ZRemRangeByScore(ctx, key, "0", formatTime(windowStart))

	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: now.UnixNano(),
	})

	countCmd := pipe.ZCard(ctx, key)

	pipe.Expire(ctx, key, time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, time.Time{}, err
	}

	count := int(countCmd.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	if count > limit {
		return false, remaining, windowEnd, nil
	}

	return true, remaining, windowEnd, nil
}

func formatTime(t time.Time) string {
	return fmt.Sprintf("%d", t.UnixNano())
}

func (r *RedisRateLimiter) Close() error {
	return r.client.Close()
}
