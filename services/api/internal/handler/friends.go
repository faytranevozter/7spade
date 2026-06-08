package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"database/sql"

	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// errResolved is a sentinel meaning resolveTarget already wrote the HTTP
// response; the caller should just return without writing again.
var errResolved = errors.New("friends: response already written")

// FriendsHandler serves the friend-graph + presence endpoints. Redis may be nil
// in tests / when unconfigured, in which case everyone reads as offline.
type FriendsHandler struct {
	DB    *sql.DB
	Redis *cache.RedisClient
}

type addFriendRequest struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type friendDTO struct {
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	Username    string  `json:"username"`
	AvatarURL   *string `json:"avatar_url"`
	Status      string  `json:"status"` // accepted | incoming | outgoing
	Online      bool    `json:"online"`
	RoomID      string  `json:"room_id,omitempty"`
}

// registeredUserID extracts the caller's user id, rejecting guests exactly like
// HistoryHandler / StatsHandler.
func (h FriendsHandler) registeredUserID(c *gin.Context) (uuid.UUID, bool) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return uuid.Nil, false
	}
	return userID, true
}

// List returns the caller's accepted friends + pending requests, each enriched
// with live presence read from Redis.
func (h FriendsHandler) List(c *gin.Context) {
	userID, ok := h.registeredUserID(c)
	if !ok {
		return
	}
	friends, err := repository.ListFriends(h.DB, userID)
	if err != nil {
		log.Printf("friends: list: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load friends")
		return
	}

	presence := h.presenceFor(c, friends)
	out := make([]friendDTO, 0, len(friends))
	for _, f := range friends {
		dto := friendDTO{
			UserID:      f.UserID,
			DisplayName: f.DisplayName,
			Username:    f.Username,
			AvatarURL:   f.AvatarURL,
			Status:      f.Status,
		}
		if p, ok := presence[f.UserID]; ok {
			dto.Online = p.Online
			dto.RoomID = p.RoomID
		}
		out = append(out, dto)
	}
	c.JSON(http.StatusOK, gin.H{"friends": out})
}

// presenceFor reads presence for the accepted friends only (pending requesters
// don't need a live dot). Returns an empty map when Redis is unavailable.
func (h FriendsHandler) presenceFor(c *gin.Context, friends []repository.FriendEntry) map[string]cache.Presence {
	if h.Redis == nil {
		return map[string]cache.Presence{}
	}
	ids := make([]string, 0, len(friends))
	for _, f := range friends {
		if f.Status == "accepted" {
			ids = append(ids, f.UserID)
		}
	}
	if len(ids) == 0 {
		return map[string]cache.Presence{}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	presence, err := h.Redis.GetPresenceBatch(ctx, ids)
	if err != nil {
		// Presence is best-effort; a Redis hiccup shouldn't fail the list.
		log.Printf("friends: presence batch: %v", err)
		return map[string]cache.Presence{}
	}
	return presence
}

// userSearchMinChars is the shortest query we'll run; shorter inputs return an
// empty result set without touching the DB (avoids whole-table scans).
const userSearchMinChars = 2

// userSearchLimit caps the number of results returned per search.
const userSearchLimit = 20

// userSearchRateLimit / userSearchRateWindow throttle searches per user. Reuses
// the Redis fixed-window helper (scope+id) so we don't need global rate-limit
// middleware (issue #53) yet.
const userSearchRateLimit = 30
const userSearchRateWindow = time.Minute

// Search finds registered users by partial username or display name for the
// add-friend flow. Auth + non-guest only; rate limited per user; the caller and
// any blocked relationships are excluded by the repository query.
func (h FriendsHandler) Search(c *gin.Context) {
	userID, ok := h.registeredUserID(c)
	if !ok {
		return
	}

	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) < userSearchMinChars {
		c.JSON(http.StatusOK, gin.H{"results": []repository.UserSearchResult{}})
		return
	}

	// Rate limit per user (best-effort; a Redis hiccup fails open).
	if h.Redis != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if allowed, err := h.Redis.AllowEmailRate(ctx, "user_search", userID.String(), userSearchRateLimit, userSearchRateWindow); err != nil {
			log.Printf("friends: search rate limit check: %v", err)
		} else if !allowed {
			JSONError(c, http.StatusTooManyRequests, "Too many searches, slow down a moment")
			return
		}
	}

	results, err := repository.SearchUsers(h.DB, query, userID, userSearchLimit)
	if err != nil {
		log.Printf("friends: search users: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to search players")
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// SendRequest sends a friend request, identified by user_id or exact (lowercase)
// username. Reverse-pending requests auto-accept.
func (h FriendsHandler) SendRequest(c *gin.Context) {
	userID, ok := h.registeredUserID(c)
	if !ok {
		return
	}
	var req addFriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	targetID, resolveErr := h.resolveTarget(c, req)
	if resolveErr != nil {
		return // resolveTarget already wrote the response
	}

	status, err := repository.SendFriendRequest(h.DB, userID, targetID)
	if err != nil {
		switch err {
		case repository.ErrFriendshipSelf:
			JSONError(c, http.StatusBadRequest, "You can't add yourself")
		case repository.ErrFriendshipBlocked:
			JSONError(c, http.StatusForbidden, "Unable to send request")
		default:
			log.Printf("friends: send request: %v", err)
			JSONError(c, http.StatusInternalServerError, "Failed to send request")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// resolveTarget turns the request body into a target user id. Writes the error
// response itself and returns a non-nil error on failure.
func (h FriendsHandler) resolveTarget(c *gin.Context, req addFriendRequest) (uuid.UUID, error) {
	if strings.TrimSpace(req.UserID) != "" {
		targetID, err := uuid.Parse(strings.TrimSpace(req.UserID))
		if err != nil {
			JSONError(c, http.StatusBadRequest, "Invalid user ID")
			return uuid.Nil, err
		}
		user, err := repository.GetUserByID(h.DB, targetID)
		if err != nil {
			log.Printf("friends: get user by id: %v", err)
			JSONError(c, http.StatusInternalServerError, "Failed to send request")
			return uuid.Nil, err
		}
		if user == nil {
			JSONError(c, http.StatusNotFound, "User not found")
			return uuid.Nil, errResolved
		}
		return targetID, nil
	}

	username := repository.NormalizeUsername(req.Username)
	if username == "" {
		JSONError(c, http.StatusBadRequest, "Provide a user_id or username")
		return uuid.Nil, errResolved
	}
	if err := repository.ValidateUsername(username); err != nil {
		JSONError(c, http.StatusNotFound, "No player with that username")
		return uuid.Nil, errResolved
	}
	user, err := repository.GetUserByUsername(h.DB, username)
	if err != nil {
		log.Printf("friends: find by username: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to send request")
		return uuid.Nil, err
	}
	if user == nil {
		JSONError(c, http.StatusNotFound, "No player with that username")
		return uuid.Nil, errResolved
	}
	return user.ID, nil
}

// Accept marks an incoming request from :userId accepted.
func (h FriendsHandler) Accept(c *gin.Context) {
	userID, ok := h.registeredUserID(c)
	if !ok {
		return
	}
	otherID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	accepted, err := repository.AcceptFriendRequest(h.DB, userID, otherID)
	if err != nil {
		log.Printf("friends: accept: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to accept request")
		return
	}
	if !accepted {
		JSONError(c, http.StatusNotFound, "No pending request from that user")
		return
	}
	c.Status(http.StatusNoContent)
}

// Remove deletes a friendship or pending request (decline / cancel / unfriend).
func (h FriendsHandler) Remove(c *gin.Context) {
	userID, ok := h.registeredUserID(c)
	if !ok {
		return
	}
	otherID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if _, err := repository.RemoveFriendship(h.DB, userID, otherID); err != nil {
		log.Printf("friends: remove: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to remove friend")
		return
	}
	c.Status(http.StatusNoContent)
}

// Block removes any relationship and records a block owned by the caller.
func (h FriendsHandler) Block(c *gin.Context) {
	userID, ok := h.registeredUserID(c)
	if !ok {
		return
	}
	otherID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if err := repository.BlockUser(h.DB, userID, otherID); err != nil {
		if err == repository.ErrFriendshipSelf {
			JSONError(c, http.StatusBadRequest, "You can't block yourself")
			return
		}
		log.Printf("friends: block: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to block user")
		return
	}
	c.Status(http.StatusNoContent)
}
