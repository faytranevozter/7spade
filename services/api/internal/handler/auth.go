package handler

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RefreshCookieName = "refresh_token"
const refreshCookieMaxAge = 30 * 24 * 60 * 60

type AuthHandler struct {
	DB        *sql.DB
	JWTSecret string
}

type guestRequest struct {
	DisplayName string `json:"display_name"`
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest carries a refresh token in the body for native clients, which
// have no cookie jar. Web clients omit the body and the HttpOnly cookie is used.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// updateMeRequest is the body for PATCH /me. Only the display name is editable.
type updateMeRequest struct {
	DisplayName string `json:"display_name"`
}

type meProviderDTO struct {
	Provider  string  `json:"provider"`
	AvatarURL *string `json:"avatar_url"`
	CreatedAt string  `json:"created_at"`
}

type meResponse struct {
	UserID      *string         `json:"user_id"`
	Username    *string         `json:"username"`
	DisplayName string          `json:"display_name"`
	AvatarURL   *string         `json:"avatar_url"`
	CreatedAt   *string         `json:"created_at"`
	IsGuest     bool            `json:"is_guest"`
	Providers   []meProviderDTO `json:"providers"`
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func (h AuthHandler) Guest(c *gin.Context) {
	var req guestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		JSONError(c, http.StatusBadRequest, "Display name is required")
		return
	}
	if len(displayName) > 50 {
		JSONError(c, http.StatusBadRequest, "Display name must be 50 characters or less")
		return
	}
	token, err := auth.GenerateGuestToken(displayName, h.JWTSecret)
	if err != nil {
		log.Printf("guest: generate token: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if !emailRegex.MatchString(email) {
		JSONError(c, http.StatusBadRequest, "Invalid email format")
		return
	}
	if len(req.Password) < 8 {
		JSONError(c, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" || len(displayName) > 50 {
		JSONError(c, http.StatusBadRequest, "Display name must be 1-50 characters")
		return
	}
	username := repository.NormalizeUsername(req.Username)
	if err := repository.ValidateUsername(username); err != nil {
		JSONError(c, http.StatusBadRequest, "Username must be 3-32 characters and use lowercase letters, numbers, or underscores")
		return
	}

	existingUser, err := repository.GetUserByEmail(h.DB, email)
	if err != nil {
		log.Printf("register: get existing user: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if existingUser != nil {
		JSONError(c, http.StatusConflict, "Email already registered")
		return
	}

	existingUsername, err := repository.GetUserByUsername(h.DB, username)
	if err != nil {
		log.Printf("register: get existing username: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if existingUsername != nil {
		JSONError(c, http.StatusConflict, "Username already taken")
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("register: hash password: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	user, err := repository.CreateUser(h.DB, email, passwordHash, displayName, username)
	if err != nil {
		if errors.Is(err, repository.ErrUsernameTaken) {
			JSONError(c, http.StatusConflict, "Username already taken")
			return
		}
		log.Printf("register: create user: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.issueAuth(c, user, http.StatusCreated)
}

func (h AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	user, err := repository.GetUserByEmail(h.DB, strings.TrimSpace(strings.ToLower(req.Email)))
	if err != nil {
		log.Printf("login: get user: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if user == nil || !user.PasswordHash.Valid || auth.ComparePassword(user.PasswordHash.String, req.Password) != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	h.issueAuth(c, user, http.StatusOK)
}

func (h AuthHandler) Refresh(c *gin.Context) {
	// Prefer the HttpOnly cookie (web); fall back to a body token (native, which
	// has no cookie jar). Track which path was used so the response echoes a
	// rotated token in the body only for native callers.
	cookie, cookieErr := c.Cookie(RefreshCookieName)
	fromBody := false
	presented := cookie
	if cookieErr != nil || presented == "" {
		var req refreshRequest
		if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
			presented = req.RefreshToken
			fromBody = true
		}
	}
	if presented == "" {
		JSONError(c, http.StatusUnauthorized, "Missing refresh token")
		return
	}
	tokenHash := auth.HashRefreshToken(presented)
	userID, err := repository.ValidateRefreshToken(h.DB, tokenHash)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}
	if err := repository.RevokeRefreshToken(h.DB, tokenHash); err != nil {
		log.Printf("refresh: revoke old token: %v", err)
	}
	user, err := repository.GetUserByID(h.DB, userID)
	if err != nil {
		log.Printf("refresh: get user: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if user == nil {
		JSONError(c, http.StatusUnauthorized, "User not found")
		return
	}

	newRefreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		log.Printf("refresh: generate refresh token: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := repository.StoreRefreshToken(h.DB, user.ID, auth.HashRefreshToken(newRefreshToken), time.Now().Add(30*24*time.Hour)); err != nil {
		log.Printf("refresh: store refresh token: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	SetRefreshCookie(c, newRefreshToken)

	avatarURL, err := repository.GetUserAvatar(h.DB, user.ID)
	if err != nil {
		log.Printf("refresh: get user avatar: %v", err)
		avatarURL = nil // non-fatal: issue the token without an avatar
	}
	jwtToken, err := auth.GenerateUserToken(user.ID.String(), user.DisplayName, derefString(avatarURL), h.JWTSecret)
	if err != nil {
		log.Printf("refresh: generate jwt: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	resp := gin.H{"jwt": jwtToken}
	if fromBody {
		resp["refresh_token"] = newRefreshToken
	}
	c.JSON(http.StatusOK, resp)
}

// Me returns account information for the authenticated session. Registered
// users get username/join date/provider links; guests get a minimal response.
func (h AuthHandler) Me(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Guests have no durable account row; return identity from the token.
	if claims.IsGuest {
		avatar := claims.AvatarURL
		resp := meResponse{
			DisplayName: claims.DisplayName,
			IsGuest:     true,
			Providers:   []meProviderDTO{},
		}
		if avatar != "" {
			resp.AvatarURL = &avatar
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return
	}

	user, err := repository.GetUserByID(h.DB, userID)
	if err != nil {
		log.Printf("me: get user by id: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if user == nil {
		JSONError(c, http.StatusUnauthorized, "User not found")
		return
	}

	avatarURL, err := repository.GetUserAvatar(h.DB, userID)
	if err != nil {
		log.Printf("me: get user avatar: %v", err)
		avatarURL = nil
	}
	providers, err := repository.ListUserProviders(h.DB, userID)
	if err != nil {
		log.Printf("me: list user providers: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	id := user.ID.String()
	username := user.Username
	createdAt := user.CreatedAt.UTC().Format(time.RFC3339)
	providerDTOs := make([]meProviderDTO, 0, len(providers))
	for _, p := range providers {
		providerDTOs = append(providerDTOs, meProviderDTO{
			Provider:  p.Provider,
			AvatarURL: p.AvatarURL,
			CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, meResponse{
		UserID:      &id,
		Username:    &username,
		DisplayName: user.DisplayName,
		AvatarURL:   avatarURL,
		CreatedAt:   &createdAt,
		IsGuest:     false,
		Providers:   providerDTOs,
	})
}

// UpdateMe updates the authenticated (registered) user's display name and
// re-issues the access JWT so the new name flows into future API calls and WS
// game sessions. The refresh token is unchanged (the name isn't stored in it),
// so this works identically for web (cookie) and native (body) clients.
//
// Note: a rename does not relabel the player's seat in an in-progress WS game —
// the seat name is captured from the JWT at connection time; it applies to the
// next connection.
func (h AuthHandler) UpdateMe(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return
	}

	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" || len(displayName) > 50 {
		JSONError(c, http.StatusBadRequest, "Display name must be 1-50 characters")
		return
	}

	user, err := repository.UpdateDisplayName(h.DB, userID, displayName)
	if err != nil {
		log.Printf("update me: update display name: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if user == nil {
		JSONError(c, http.StatusUnauthorized, "User not found")
		return
	}

	avatarURL, err := repository.GetUserAvatar(h.DB, user.ID)
	if err != nil {
		log.Printf("update me: get user avatar: %v", err)
		avatarURL = nil // non-fatal: re-issue the token without an avatar
	}
	jwtToken, err := auth.GenerateUserToken(user.ID.String(), user.DisplayName, derefString(avatarURL), h.JWTSecret)
	if err != nil {
		log.Printf("update me: generate jwt: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"jwt": jwtToken})
}

func (h AuthHandler) Logout(c *gin.Context) {
	// Revoke the cookie token (web) and/or a body token (native).
	if cookie, err := c.Cookie(RefreshCookieName); err == nil && cookie != "" {
		if err := repository.RevokeRefreshToken(h.DB, auth.HashRefreshToken(cookie)); err != nil {
			log.Printf("logout: revoke refresh token: %v", err)
		}
	}
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		if err := repository.RevokeRefreshToken(h.DB, auth.HashRefreshToken(req.RefreshToken)); err != nil {
			log.Printf("logout: revoke body refresh token: %v", err)
		}
	}
	ClearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

func (h AuthHandler) issueAuth(c *gin.Context, user *repository.User, status int) {
	avatarURL, err := repository.GetUserAvatar(h.DB, user.ID)
	if err != nil {
		log.Printf("auth: get user avatar: %v", err)
		avatarURL = nil // non-fatal: issue the token without an avatar
	}
	jwtToken, err := auth.GenerateUserToken(user.ID.String(), user.DisplayName, derefString(avatarURL), h.JWTSecret)
	if err != nil {
		log.Printf("auth: generate jwt: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		log.Printf("auth: generate refresh token: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := repository.StoreRefreshToken(h.DB, user.ID, auth.HashRefreshToken(refreshToken), time.Now().Add(30*24*time.Hour)); err != nil {
		log.Printf("auth: store refresh token: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	SetRefreshCookie(c, refreshToken)
	c.JSON(status, gin.H{"jwt": jwtToken})
}

// derefString returns the pointed-to string, or "" when nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func SetRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(RefreshCookieName, token, refreshCookieMaxAge, "/", "", c.Request.TLS != nil, true)
}

func ClearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(RefreshCookieName, "", -1, "/", "", c.Request.TLS != nil, true)
}
