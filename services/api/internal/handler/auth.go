package handler

import (
	"database/sql"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
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
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("register: hash password: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	user, err := repository.CreateUser(h.DB, email, passwordHash, displayName)
	if err != nil {
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
	cookie, err := c.Cookie(RefreshCookieName)
	if err != nil || cookie == "" {
		JSONError(c, http.StatusUnauthorized, "Missing refresh token")
		return
	}
	tokenHash := auth.HashRefreshToken(cookie)
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
	c.JSON(http.StatusOK, gin.H{"jwt": jwtToken})
}

func (h AuthHandler) Logout(c *gin.Context) {
	if cookie, err := c.Cookie(RefreshCookieName); err == nil && cookie != "" {
		if err := repository.RevokeRefreshToken(h.DB, auth.HashRefreshToken(cookie)); err != nil {
			log.Printf("logout: revoke refresh token: %v", err)
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
