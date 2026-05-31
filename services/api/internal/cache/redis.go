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

// presenceKey is the key the WS service writes (with a TTL) while a user is
// connected. The value is the user's current room_id (or "" when not in a
// room). Both services must agree on this format.
func presenceKey(userID string) string { return "presence:user:" + userID }

// Presence is a user's live presence snapshot read from Redis.
type Presence struct {
	Online bool
	RoomID string
}

// GetPresenceBatch reads presence for many users in one round-trip. A missing
// key means offline. The value (when present) is the user's current room_id, or
// empty when they're online but not in a room.
func (r *RedisClient) GetPresenceBatch(ctx context.Context, userIDs []string) (map[string]Presence, error) {
	result := make(map[string]Presence, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = presenceKey(id)
	}
	vals, err := r.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("cache: mget presence: %w", err)
	}
	for i, v := range vals {
		if v == nil {
			result[userIDs[i]] = Presence{Online: false}
			continue
		}
		roomID, _ := v.(string)
		result[userIDs[i]] = Presence{Online: true, RoomID: roomID}
	}
	return result, nil
}
