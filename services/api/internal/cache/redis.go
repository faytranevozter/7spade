package cache

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

type oauthStateValue struct {
	CodeVerifier string `json:"code_verifier"`
	Provider     string `json:"provider"`
}

// New parses redisURL and returns a connected RedisClient.
func New(redisURL string) (*RedisClient, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid Redis URL: %w", err)
	}
	return &RedisClient{rdb: redis.NewClient(opts)}, nil
}

// Ping checks connectivity.
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.rdb.Ping(ctx).Err()
}

// Close closes the underlying connection.
func (r *RedisClient) Close() error {
	return r.rdb.Close()
}

// StoreOAuthState stores {state → codeVerifier, provider} with the given TTL.
func (r *RedisClient) StoreOAuthState(ctx context.Context, state, codeVerifier, provider string, ttl time.Duration) error {
	data, err := json.Marshal(oauthStateValue{CodeVerifier: codeVerifier, Provider: provider})
	if err != nil {
		return fmt.Errorf("cache: marshal oauth state: %w", err)
	}
	return r.rdb.Set(ctx, oauthStateKey(state), data, ttl).Err()
}

// GetAndDeleteOAuthState atomically fetches and removes a state entry (one-time use).
// Returns ErrStateNotFound when the state is missing or expired.
func (r *RedisClient) GetAndDeleteOAuthState(ctx context.Context, state string) (codeVerifier, provider string, err error) {
	key := oauthStateKey(state)

	pipe := r.rdb.TxPipeline()
	getCmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return "", "", fmt.Errorf("cache: exec get+del: %w", err)
	}

	raw, err := getCmd.Result()
	if err == redis.Nil {
		return "", "", fmt.Errorf("cache: oauth state not found or expired")
	}
	if err != nil {
		return "", "", fmt.Errorf("cache: get oauth state: %w", err)
	}

	var val oauthStateValue
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return "", "", fmt.Errorf("cache: unmarshal oauth state: %w", err)
	}
	return val.CodeVerifier, val.Provider, nil
}

func oauthStateKey(state string) string { return "oauth:state:" + state }
