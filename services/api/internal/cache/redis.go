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
	// RedirectURI is the redirect_uri used in the authorize request. It must be
	// replayed verbatim in the token exchange. Empty means "use the provider's
	// configured default" (the web flow); native clients store their deep-link
	// URI here.
	RedirectURI string `json:"redirect_uri,omitempty"`
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

// StoreOAuthState stores {state → codeVerifier, provider, redirectURI} with the
// given TTL. redirectURI may be empty to use the provider's configured default.
func (r *RedisClient) StoreOAuthState(ctx context.Context, state, codeVerifier, provider, redirectURI string, ttl time.Duration) error {
	data, err := json.Marshal(oauthStateValue{CodeVerifier: codeVerifier, Provider: provider, RedirectURI: redirectURI})
	if err != nil {
		return fmt.Errorf("cache: marshal oauth state: %w", err)
	}
	return r.rdb.Set(ctx, oauthStateKey(state), data, ttl).Err()
}

// GetAndDeleteOAuthState atomically fetches and removes a state entry (one-time use).
// Returns an error when the state is missing or expired.
func (r *RedisClient) GetAndDeleteOAuthState(ctx context.Context, state string) (codeVerifier, provider, redirectURI string, err error) {
	key := oauthStateKey(state)

	pipe := r.rdb.TxPipeline()
	getCmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return "", "", "", fmt.Errorf("cache: exec get+del: %w", err)
	}

	raw, err := getCmd.Result()
	if err == redis.Nil {
		return "", "", "", fmt.Errorf("cache: oauth state not found or expired")
	}
	if err != nil {
		return "", "", "", fmt.Errorf("cache: get oauth state: %w", err)
	}

	var val oauthStateValue
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return "", "", "", fmt.Errorf("cache: unmarshal oauth state: %w", err)
	}
	return val.CodeVerifier, val.Provider, val.RedirectURI, nil
}

func oauthStateKey(state string) string { return "oauth:state:" + state }

// --- Emailed single-use tokens (password reset, email verification) ---
//
// Only the SHA-256 hash of the token is stored as the key, so a Redis dump
// cannot be replayed as a valid link. The value is the user id. Tokens are
// single-use: consumption atomically reads and deletes the key.

func passwordResetKey(tokenHash string) string { return "password_reset:" + tokenHash }
func emailVerifyKey(tokenHash string) string   { return "email_verify:" + tokenHash }

// StorePasswordResetToken stores tokenHash -> userID with the given TTL.
func (r *RedisClient) StorePasswordResetToken(ctx context.Context, tokenHash, userID string, ttl time.Duration) error {
	if err := r.rdb.Set(ctx, passwordResetKey(tokenHash), userID, ttl).Err(); err != nil {
		return fmt.Errorf("cache: store password reset token: %w", err)
	}
	return nil
}

// ConsumePasswordResetToken atomically fetches and deletes a reset token,
// returning the associated user id. Returns an error when the token is missing
// or expired (single-use).
func (r *RedisClient) ConsumePasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	return r.consumeToken(ctx, passwordResetKey(tokenHash))
}

// StoreEmailVerifyToken stores tokenHash -> userID with the given TTL.
func (r *RedisClient) StoreEmailVerifyToken(ctx context.Context, tokenHash, userID string, ttl time.Duration) error {
	if err := r.rdb.Set(ctx, emailVerifyKey(tokenHash), userID, ttl).Err(); err != nil {
		return fmt.Errorf("cache: store email verify token: %w", err)
	}
	return nil
}

// ConsumeEmailVerifyToken atomically fetches and deletes a verification token,
// returning the associated user id. Returns an error when missing or expired.
func (r *RedisClient) ConsumeEmailVerifyToken(ctx context.Context, tokenHash string) (string, error) {
	return r.consumeToken(ctx, emailVerifyKey(tokenHash))
}

// consumeToken atomically reads and deletes a key, returning its value.
func (r *RedisClient) consumeToken(ctx context.Context, key string) (string, error) {
	pipe := r.rdb.TxPipeline()
	getCmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return "", fmt.Errorf("cache: exec get+del: %w", err)
	}
	val, err := getCmd.Result()
	if err == redis.Nil {
		return "", fmt.Errorf("cache: token not found or expired")
	}
	if err != nil {
		return "", fmt.Errorf("cache: get token: %w", err)
	}
	return val, nil
}

// --- Fixed-window rate limiting (auth tiers, social, email recovery, …) ---

func rateLimitKey(scope, subject string) string { return "rate:" + scope + ":" + subject }

// RateResult is the outcome of a fixed-window allow check.
type RateResult struct {
	// Allowed is true when the action may proceed (and was counted).
	Allowed bool
	// RetryAfter is how long until the window resets when Allowed is false.
	// Zero when Allowed is true or TTL is unknown.
	RetryAfter time.Duration
}

// AllowRate enforces a fixed-window rate limit of `limit` actions per `window`
// for scope+subject. The window starts on the first action and is reset by
// Redis key expiry (INCR + EXPIRE on first hit). On a Redis error it fails OPEN
// (Allowed=true) so a cache blip can't lock clients out.
func (r *RedisClient) AllowRate(ctx context.Context, scope, subject string, limit int, window time.Duration) (RateResult, error) {
	if r == nil || r.rdb == nil {
		return RateResult{Allowed: true}, nil
	}
	if limit <= 0 {
		return RateResult{Allowed: true}, nil
	}
	key := rateLimitKey(scope, subject)
	count, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return RateResult{Allowed: true}, fmt.Errorf("cache: incr rate limit: %w", err)
	}
	if count == 1 {
		if err := r.rdb.Expire(ctx, key, window).Err(); err != nil {
			return RateResult{Allowed: true}, fmt.Errorf("cache: expire rate limit: %w", err)
		}
	}
	if count <= int64(limit) {
		return RateResult{Allowed: true}, nil
	}
	ttl, err := r.rdb.TTL(ctx, key).Result()
	if err != nil {
		// Counted over limit; still deny, but Retry-After may be approximate.
		return RateResult{Allowed: false, RetryAfter: window}, fmt.Errorf("cache: ttl rate limit: %w", err)
	}
	if ttl <= 0 {
		ttl = window
	}
	return RateResult{Allowed: false, RetryAfter: ttl}, nil
}

// AllowEmailRate is a convenience wrapper around AllowRate for per-email scopes
// (password reset / verification). Returns only the allow bit for existing callers.
func (r *RedisClient) AllowEmailRate(ctx context.Context, scope, email string, limit int, window time.Duration) (bool, error) {
	res, err := r.AllowRate(ctx, scope, email, limit, window)
	return res.Allowed, err
}

// AllowOnce enforces a one-action-per-window cooldown for a subject. It returns
// true when the action is allowed and false when a previous action is still in
// its cooldown window. On Redis errors it fails open, matching AllowRate.
func (r *RedisClient) AllowOnce(ctx context.Context, scope, subject string, window time.Duration) (bool, error) {
	if r == nil || r.rdb == nil {
		return true, nil
	}
	key := rateLimitKey(scope, subject)
	ok, err := r.rdb.SetNX(ctx, key, "1", window).Result()
	if err != nil {
		return true, fmt.Errorf("cache: set cooldown: %w", err)
	}
	return ok, nil
}

// CooldownRemaining returns the remaining cooldown for a prior AllowOnce key,
// or 0 when the key is absent / expired. Fail-open returns 0 on Redis errors.
func (r *RedisClient) CooldownRemaining(ctx context.Context, scope, subject string) (time.Duration, error) {
	if r == nil || r.rdb == nil {
		return 0, nil
	}
	ttl, err := r.rdb.TTL(ctx, rateLimitKey(scope, subject)).Result()
	if err != nil {
		return 0, fmt.Errorf("cache: ttl cooldown: %w", err)
	}
	if ttl < 0 {
		return 0, nil
	}
	return ttl, nil
}

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
