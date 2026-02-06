package circuitbreaker

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/redis/go-redis/v9"
)

// Lua scripts for atomic circuit breaker operations.
// These scripts ensure that state transitions are atomic across multiple Redis keys.

// allowScript checks if a request should be allowed and handles state transitions.
// Keys: [state_key, last_failure_key, successes_key]
// Args: [timeout_seconds]
// Returns: current state as string
var allowScript = redis.NewScript(`
local state = redis.call('GET', KEYS[1]) or 'closed'
local timeout = tonumber(ARGV[1])

if state == 'open' then
    local lastFailure = tonumber(redis.call('GET', KEYS[2]) or '0')
    local now = tonumber(redis.call('TIME')[1])
    
    if (now - lastFailure) >= timeout then
        redis.call('SET', KEYS[1], 'half-open')
        redis.call('SET', KEYS[3], '0')
        return 'half-open'
    end
    return 'open'
end

return state
`)

// recordSuccessScript records a successful request and handles state transitions.
// Keys: [state_key, failures_key, successes_key]
// Args: [success_threshold]
// Returns: new state as string
var recordSuccessScript = redis.NewScript(`
local state = redis.call('GET', KEYS[1]) or 'closed'

if state == 'closed' then
    redis.call('SET', KEYS[2], '0')
    return 'closed'
end

if state == 'half-open' then
    local successes = redis.call('INCR', KEYS[3])
    local threshold = tonumber(ARGV[1])
    
    if successes >= threshold then
        redis.call('SET', KEYS[1], 'closed')
        redis.call('SET', KEYS[2], '0')
        redis.call('SET', KEYS[3], '0')
        return 'closed'
    end
    return 'half-open'
end

return state
`)

// recordFailureScript records a failed request and handles state transitions.
// Keys: [state_key, failures_key, last_failure_key, successes_key]
// Args: [failure_threshold]
// Returns: new state as string
var recordFailureScript = redis.NewScript(`
local state = redis.call('GET', KEYS[1]) or 'closed'
local now = redis.call('TIME')[1]

redis.call('SET', KEYS[3], now)

if state == 'closed' then
    local failures = redis.call('INCR', KEYS[2])
    local threshold = tonumber(ARGV[1])
    
    if failures >= threshold then
        redis.call('SET', KEYS[1], 'open')
        return 'open'
    end
    return 'closed'
end

if state == 'half-open' then
    redis.call('SET', KEYS[1], 'open')
    redis.call('SET', KEYS[4], '0')
    return 'open'
end

return state
`)

// RedisCircuitBreaker implements a distributed circuit breaker using Redis.
// It uses Lua scripts for atomic state transitions, ensuring consistency
// across multiple gateway instances.
type RedisCircuitBreaker struct {
	client     *redis.Client
	providerID string
	config     Config
	keyPrefix  string
}

// NewRedis creates a new Redis-backed circuit breaker.
func NewRedis(redisURL string, providerID string, cfg Config) (*RedisCircuitBreaker, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &RedisCircuitBreaker{
		client:     client,
		providerID: providerID,
		config:     cfg,
		keyPrefix:  fmt.Sprintf("cb:%s:", providerID),
	}, nil
}

// NewRedisWithClient creates a new Redis-backed circuit breaker with an existing client.
// Useful for sharing a Redis connection pool across multiple circuit breakers.
func NewRedisWithClient(client *redis.Client, providerID string, cfg Config) *RedisCircuitBreaker {
	return &RedisCircuitBreaker{
		client:     client,
		providerID: providerID,
		config:     cfg,
		keyPrefix:  fmt.Sprintf("cb:%s:", providerID),
	}
}

func (cb *RedisCircuitBreaker) stateKey() string {
	return cb.keyPrefix + "state"
}

func (cb *RedisCircuitBreaker) failuresKey() string {
	return cb.keyPrefix + "failures"
}

func (cb *RedisCircuitBreaker) successesKey() string {
	return cb.keyPrefix + "successes"
}

func (cb *RedisCircuitBreaker) lastFailureKey() string {
	return cb.keyPrefix + "last_failure"
}

// Allow checks if a request should be allowed through.
// Uses a Lua script for atomic state check and transition from open to half-open.
func (cb *RedisCircuitBreaker) Allow(ctx context.Context) error {
	keys := []string{
		cb.stateKey(),
		cb.lastFailureKey(),
		cb.successesKey(),
	}
	args := []interface{}{
		int(cb.config.Timeout.Seconds()),
	}

	result, err := allowScript.Run(ctx, cb.client, keys, args...).Text()
	if err != nil {
		// On Redis error, fail open (allow the request)
		return nil
	}

	if result == "open" {
		return domain.ErrCircuitBreakerOpen
	}

	return nil
}

// RecordSuccess records a successful request.
// Uses a Lua script for atomic state transition from half-open to closed.
func (cb *RedisCircuitBreaker) RecordSuccess(ctx context.Context) {
	keys := []string{
		cb.stateKey(),
		cb.failuresKey(),
		cb.successesKey(),
	}
	args := []interface{}{
		cb.config.SuccessThreshold,
	}

	recordSuccessScript.Run(ctx, cb.client, keys, args...)
}

// RecordFailure records a failed request.
// Uses a Lua script for atomic failure counting and state transition.
func (cb *RedisCircuitBreaker) RecordFailure(ctx context.Context) {
	keys := []string{
		cb.stateKey(),
		cb.failuresKey(),
		cb.lastFailureKey(),
		cb.successesKey(),
	}
	args := []interface{}{
		cb.config.FailureThreshold,
	}

	recordFailureScript.Run(ctx, cb.client, keys, args...)
}

// State returns the current state of the circuit breaker.
func (cb *RedisCircuitBreaker) State(ctx context.Context) State {
	result, err := cb.client.Get(ctx, cb.stateKey()).Result()
	if err != nil {
		// Default to closed on error
		return StateClosed
	}

	return parseState(result)
}

// Failures returns the current failure count.
func (cb *RedisCircuitBreaker) Failures(ctx context.Context) int {
	result, err := cb.client.Get(ctx, cb.failuresKey()).Result()
	if err != nil {
		return 0
	}

	failures, _ := strconv.Atoi(result)
	return failures
}

// Reset resets the circuit breaker to closed state.
// Useful for manual intervention or testing.
func (cb *RedisCircuitBreaker) Reset(ctx context.Context) error {
	pipe := cb.client.Pipeline()
	pipe.Set(ctx, cb.stateKey(), "closed", 0)
	pipe.Set(ctx, cb.failuresKey(), "0", 0)
	pipe.Set(ctx, cb.successesKey(), "0", 0)
	pipe.Del(ctx, cb.lastFailureKey())
	_, err := pipe.Exec(ctx)
	return err
}

// Close closes the Redis client connection.
func (cb *RedisCircuitBreaker) Close() error {
	return cb.client.Close()
}

func parseState(s string) State {
	switch s {
	case "open":
		return StateOpen
	case "half-open":
		return StateHalfOpen
	default:
		return StateClosed
	}
}
