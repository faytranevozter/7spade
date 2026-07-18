package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type deleteAccountRequest struct {
	Password string `json:"password"`
}

// DeleteAccount schedules account deletion after identity confirmation.
// Email/password users must supply the current password; OAuth-only users
// (no password_hash) may delete without a password. Guests are rejected.
// Already-pending schedules are idempotent (200 with existing timestamp).
// On success (new or idempotent), all refresh tokens are revoked and the
// refresh cookie is cleared; the access JWT remains valid so the client can
// cancel during the grace period.
func (h AuthHandler) DeleteAccount(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil || claims.IsGuest {
		JSONError(c, http.StatusForbidden, "Account deletion is only available for registered users")
		return
	}

	user, err := repository.GetUserByID(h.DB, userID)
	if err != nil {
		log.Printf("delete account: get user: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if user == nil {
		JSONError(c, http.StatusUnauthorized, "User not found")
		return
	}

	var req deleteAccountRequest
	// Empty body is fine for OAuth-only; bind errors only matter when body is present and invalid.
	_ = c.ShouldBindJSON(&req)

	if user.PasswordHash.Valid {
		if req.Password == "" {
			JSONError(c, http.StatusBadRequest, "Password is required")
			return
		}
		if auth.ComparePassword(user.PasswordHash.String, req.Password) != nil {
			JSONError(c, http.StatusUnauthorized, "Invalid password")
			return
		}
	}

	scheduledAt, newlyScheduled, err := repository.ScheduleAccountDeletion(h.DB, userID)
	if err != nil {
		log.Printf("delete account: schedule: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if scheduledAt.IsZero() {
		JSONError(c, http.StatusUnauthorized, "User not found")
		return
	}

	if err := repository.RevokeAllRefreshTokensForUser(h.DB, userID); err != nil {
		log.Printf("delete account: revoke refresh tokens: %v", err)
	}
	ClearRefreshCookie(c)

	if newlyScheduled {
		log.Printf("account_deletion: schedule user_id=%s at=%s", userID, scheduledAt.UTC().Format(time.RFC3339))
	} else {
		log.Printf("account_deletion: schedule_idempotent user_id=%s at=%s", userID, scheduledAt.UTC().Format(time.RFC3339))
	}

	c.JSON(http.StatusOK, gin.H{
		"deletion_scheduled_at": scheduledAt.UTC().Format(time.RFC3339),
		"grace_days":            int(repository.AccountDeletionGracePeriod.Hours() / 24),
	})
}

// CancelDeletion clears a pending deletion schedule within the grace window.
func (h AuthHandler) CancelDeletion(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil || claims.IsGuest {
		JSONError(c, http.StatusForbidden, "Only registered users can cancel account deletion")
		return
	}

	cleared, err := repository.CancelAccountDeletion(h.DB, userID)
	if err != nil {
		log.Printf("cancel deletion: %v", err)
		JSONError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if !cleared {
		JSONError(c, http.StatusConflict, "No pending account deletion")
		return
	}

	log.Printf("account_deletion: cancel user_id=%s", userID)
	c.Status(http.StatusOK)
	c.JSON(http.StatusOK, gin.H{"cancelled": true})
}
