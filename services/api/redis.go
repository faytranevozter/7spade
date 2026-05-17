package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the go-redis client with domain-specific helpers.
type RedisClient struct {
	rdb *redis.Client
}

// oauthStateValue is the JSON payload stored in Redis for an in-flight OAuth state.
type oauthStateValue struct {
	CodeVerifier string `json:"code_verifier"`
	Provider     string `json:"provider"`
}

// NewRedisClient connects to Redis using the given URL.
func NewRedisClient(redisURL string) (*RedisClient, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: invalid URL: %w", err)
	}
	rdb := redis.NewClient(opts)
	return &RedisClient{rdb: rdb}, nil
}

// Ping checks the Redis connection.
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.rdb.Ping(ctx).Err()
}

// Close closes the underlying Redis connection.
func (r *RedisClient) Close() error {
	return r.rdb.Close()
}

// StoreOAuthState persists the state→{codeVerifier, provider} mapping with a TTL.
func (r *RedisClient) StoreOAuthState(ctx context.Context, state, codeVerifier, provider string, ttl time.Duration) error {
	val := oauthStateValue{CodeVerifier: codeVerifier, Provider: provider}
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("redis: marshal oauth state: %w", err)
	}
	key := oauthStateKey(state)
	return r.rdb.Set(ctx, key, data, ttl).Err()
}

// GetAndDeleteOAuthState fetches and atomically deletes the state entry.
// Returns the code_verifier and provider. Returns an error if the state is not found.
func (r *RedisClient) GetAndDeleteOAuthState(ctx context.Context, state string) (codeVerifier, provider string, err error) {
	key := oauthStateKey(state)

	pipe := r.rdb.TxPipeline()
	getCmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return "", "", fmt.Errorf("redis: exec get+del: %w", err)
	}

	raw, err := getCmd.Result()
	if err == redis.Nil {
		return "", "", fmt.Errorf("oauth state not found or expired")
	}
	if err != nil {
		return "", "", fmt.Errorf("redis: get oauth state: %w", err)
	}

	var val oauthStateValue
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return "", "", fmt.Errorf("redis: unmarshal oauth state: %w", err)
	}
	return val.CodeVerifier, val.Provider, nil
}

func oauthStateKey(state string) string {
	return "oauth:state:" + state
}
