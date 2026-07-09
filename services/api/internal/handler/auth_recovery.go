package handler

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	passwordResetTTL = 15 * time.Minute
	emailVerifyTTL   = 24 * time.Hour

	// Per-email rate limits (fixed 1-hour window).
	passwordResetRateLimit = 3
	verifyEmailRateLimit   = 5
	emailRateWindow        = time.Hour
)

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

// ForgotPassword starts a password reset. It ALWAYS returns 200 (even when the
// email is unknown or has no password set) so the endpoint can't be used to
// enumerate registered emails. A reset link is emailed only when a matching
// password account exists.
func (h AuthHandler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	// Always respond 200 regardless of outcome below.
	defer c.JSON(http.StatusOK, gin.H{"message": "If an account exists, a reset link has been sent."})

	if email == "" || h.Redis == nil || h.Email == nil {
		return
	}
	user, err := repository.GetUserByEmail(h.DB, email)
	if err != nil {
		log.Printf("forgot-password: get user: %v", err)
		return
	}
	// Only password accounts can reset a password (OAuth-only users have none).
	if user == nil || !user.PasswordHash.Valid {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	// Rate limit per email (max 3/hr). Checked before minting/sending so abuse
	// can't spam an inbox; still returns 200 above to avoid enumeration.
	if allowed, err := h.Redis.AllowEmailRate(ctx, "pwreset", email, passwordResetRateLimit, emailRateWindow); err != nil {
		log.Printf("forgot-password: rate limit check: %v", err)
	} else if !allowed {
		log.Printf("forgot-password: rate limit reached for %s", email)
		return
	}

	token, err := auth.GenerateURLToken()
	if err != nil {
		log.Printf("forgot-password: generate token: %v", err)
		return
	}
	if err := h.Redis.StorePasswordResetToken(ctx, auth.HashToken(token), user.ID.String(), passwordResetTTL); err != nil {
		log.Printf("forgot-password: store token: %v", err)
		return
	}
	link := h.frontendLink("/reset-password", token)
	if err := h.Email.SendPasswordReset(ctx, email, link); err != nil {
		log.Printf("forgot-password: send email: %v", err)
	}
}

// ResetPassword consumes a single-use reset token, sets the new bcrypt hash, and
// revokes all of the user's refresh tokens (logging out every session).
func (h AuthHandler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Token) == "" {
		JSONError(c, http.StatusBadRequest, "Reset token is required")
		return
	}
	if len(req.Password) < 8 {
		JSONError(c, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}
	if len(req.Password) > auth.MaxPasswordBytes {
		JSONError(c, http.StatusBadRequest, "Password must be 72 bytes or fewer")
		return
	}
	if h.Redis == nil {
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	userIDStr, err := h.Redis.ConsumePasswordResetToken(ctx, auth.HashToken(req.Token))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid or expired reset token")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid or expired reset token")
		return
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("reset-password: hash: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := repository.UpdatePasswordHash(h.DB, userID, passwordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			JSONError(c, http.StatusBadRequest, "Invalid or expired reset token")
			return
		}
		log.Printf("reset-password: update hash: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := repository.RevokeAllRefreshTokensForUser(h.DB, userID); err != nil {
		log.Printf("reset-password: revoke tokens: %v", err)
	}
	ClearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "Password updated. Please sign in with your new password."})
}

// VerifyEmail consumes a single-use verification token and marks the account's
// email as verified.
func (h AuthHandler) VerifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Token) == "" {
		JSONError(c, http.StatusBadRequest, "Verification token is required")
		return
	}
	if h.Redis == nil {
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	userIDStr, err := h.Redis.ConsumeEmailVerifyToken(ctx, auth.HashToken(req.Token))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid or expired verification token")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid or expired verification token")
		return
	}
	if err := repository.MarkEmailVerified(h.DB, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			JSONError(c, http.StatusBadRequest, "Invalid or expired verification token")
			return
		}
		log.Printf("verify-email: mark verified: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Email verified."})
}

// ResendVerification re-sends the verification email for the authenticated user
// (when not already verified). Always returns 204 so it leaks no account state.
func (h AuthHandler) ResendVerification(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	defer c.Status(http.StatusNoContent)

	if claims.IsGuest {
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		return
	}
	user, err := repository.GetUserByID(h.DB, userID)
	if err != nil || user == nil || !user.Email.Valid || user.EmailVerifiedAt.Valid {
		return
	}
	// Rate limit resends per email (max 5/hr). Registration's initial send is
	// not limited (it happens once); only user-triggered resends are.
	if h.Redis != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		if allowed, rerr := h.Redis.AllowEmailRate(ctx, "verify", user.Email.String, verifyEmailRateLimit, emailRateWindow); rerr != nil {
			log.Printf("resend-verification: rate limit check: %v", rerr)
		} else if !allowed {
			log.Printf("resend-verification: rate limit reached for %s", user.Email.String)
			return
		}
	}
	h.sendVerificationEmail(c.Request.Context(), user.ID, user.Email.String)
}

// sendVerificationEmail mints a verification token, stores its hash in Redis, and
// emails the link. All failures are logged and swallowed (verification is soft;
// callers must not fail their primary flow because of it).
func (h AuthHandler) sendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) {
	if h.Redis == nil || h.Email == nil || email == "" {
		return
	}
	token, err := auth.GenerateURLToken()
	if err != nil {
		log.Printf("verify-email: generate token: %v", err)
		return
	}
	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := h.Redis.StoreEmailVerifyToken(sctx, auth.HashToken(token), userID.String(), emailVerifyTTL); err != nil {
		log.Printf("verify-email: store token: %v", err)
		return
	}
	link := h.frontendLink("/verify-email", token)
	if err := h.Email.SendVerification(sctx, email, link); err != nil {
		log.Printf("verify-email: send email: %v", err)
	}
}

// frontendLink builds a single link format pointing at the web app. Native
// clients handle the same token via their own deep-linked screens.
func (h AuthHandler) frontendLink(path, token string) string {
	base := strings.TrimRight(h.FrontendURL, "/")
	return base + path + "?token=" + url.QueryEscape(token)
}
