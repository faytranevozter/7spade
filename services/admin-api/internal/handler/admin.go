package handler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	refreshCookieName = "admin_refresh_token"
	csrfCookieName    = "admin_csrf_token"
	recoveryCodeCount = 8
	auditExportLimit  = 10_000
)

type Config struct {
	JWTSecret        string
	SecureCookies    bool
	AccessTTL        time.Duration
	RefreshTTL       time.Duration
	MFAEncryptionKey string
	Environment      string
}

type (
	Admin                       = model.Admin
	Role                        = model.Role
	Permission                  = model.Permission
	Invitation                  = model.Invitation
	Session                     = model.Session
	AuditEvent                  = model.AuditEvent
	AuditFilter                 = model.AuditFilter
	AuditEventPage              = model.AuditEventPage
	Dashboard                   = model.Dashboard
	User                        = model.User
	UserPage                    = model.UserPage
	UserDetail                  = model.UserDetail
	Suspension                  = model.Suspension
	Room                        = model.Room
	RoomPlayer                  = model.RoomPlayer
	RoomFilter                  = model.RoomFilter
	RoomPage                    = model.RoomPage
	RoomDetail                  = model.RoomDetail
	LiveRoomPlayer              = model.LiveRoomPlayer
	LiveRoomSummary             = model.LiveRoomSummary
	RoomInvestigation           = model.RoomInvestigation
	Game                        = model.Game
	GamePlayer                  = model.GamePlayer
	GameMove                    = model.GameMove
	GameCard                    = model.GameCard
	GameFlag                    = model.GameFlag
	GameNote                    = model.GameNote
	GameDetail                  = model.GameDetail
	GameFilter                  = model.GameFilter
	GamePage                    = model.GamePage
	Achievement                 = model.Achievement
	AchievementRule             = model.AchievementRule
	AchievementEntitlementEvent = model.AchievementEntitlementEvent
)

var (
	ErrNotFound   = model.ErrNotFound
	ErrConflict   = model.ErrConflict
	ErrUserActive = model.ErrUserActive
)

type Store interface {
	FindAdminByEmail(context.Context, string) (Admin, error)
	FindAdminByID(context.Context, string) (Admin, error)
	ListAdmins(context.Context) ([]Admin, error)
	CreateInvitation(context.Context, Invitation, AuditEvent) error
	FindInvitationByTokenHash(context.Context, string) (Invitation, error)
	ListInvitations(context.Context) ([]Invitation, error)
	RevokeInvitation(context.Context, string, AuditEvent) error
	ReissueInvitation(context.Context, string, string, time.Time, AuditEvent) (Invitation, error)
	AcceptInvitation(context.Context, string, string, string, AuditEvent) (Admin, error)
	SetAdminStatus(context.Context, string, string, AuditEvent) error
	SetAdminRoles(context.Context, string, []string, AuditEvent) error
	ListRoles(context.Context) ([]Role, error)
	ListPermissions(context.Context) ([]Permission, error)
	UpdateRolePermissions(context.Context, string, []string, AuditEvent) error
	GetFeatureSetting(context.Context, string) (model.FeatureSetting, error)
	ListFeatureSettings(context.Context) ([]model.FeatureSetting, error)
	UpdateFeatureSetting(context.Context, string, bool, AuditEvent) (model.FeatureSetting, error)
	RecordLoginFailure(context.Context, string) error
	RecordLoginSuccess(context.Context, string) error
	CreateSession(context.Context, Session) error
	RotateSession(context.Context, string, string, string, time.Time) (Session, error)
	RevokeSession(context.Context, string) error
	RevokeAdminSessions(context.Context, string) error
	ListSessions(context.Context, string) ([]Session, error)
	RevokeSessionByID(context.Context, string, string, AuditEvent) (bool, error)
	RevokeOtherSessions(context.Context, string, string, AuditEvent) error
	SessionActive(context.Context, string, string) (Session, error)
	SavePendingMFA(context.Context, string, []byte) error
	MFASecret(context.Context, string, bool) ([]byte, error)
	ConfirmMFA(context.Context, string, []string) error
	UseRecoveryCode(context.Context, string, string) (bool, error)
	AppendAudit(context.Context, AuditEvent) error
	ListAuditEvents(context.Context, AuditFilter) (AuditEventPage, error)
	Dashboard(context.Context) (Dashboard, error)
	SearchUsers(context.Context, string, int, int, bool) (UserPage, error)
	GetUser(context.Context, string, bool) (UserDetail, error)
	SuspendUser(context.Context, string, Suspension, AuditEvent) error
	ReinstateUser(context.Context, string, AuditEvent) error
	UpdateUserDisplayName(context.Context, string, string, int, AuditEvent) (User, error)
	SearchRooms(context.Context, RoomFilter) (RoomPage, error)
	GetRoom(context.Context, string) (RoomDetail, error)
	SearchGames(context.Context, GameFilter) (GamePage, error)
	GetGame(context.Context, string) (GameDetail, error)
	FlagGame(context.Context, string, string, AuditEvent) (GameFlag, error)
	AddGameNote(context.Context, string, string, string, AuditEvent) (GameNote, error)
	ListSkins(context.Context) ([]Skin, error)
	GetSkin(context.Context, string) (Skin, error)
	ListAchievements(context.Context) ([]model.Achievement, error)
	CreateAchievement(context.Context, model.Achievement, AuditEvent) (model.Achievement, error)
	UpdateAchievement(context.Context, string, model.Achievement, AuditEvent) (model.Achievement, error)
	ChangeAchievementEntitlement(context.Context, string, string, string, string, string, AuditEvent) (model.AchievementEntitlementEvent, bool, error)
	ListEvents(context.Context) ([]model.Event, error)
	GetEvent(context.Context, string) (model.Event, error)
	CreateEvent(context.Context, model.Event, AuditEvent) (model.Event, error)
	UpdateEvent(context.Context, string, int, model.Event, AuditEvent) (model.Event, error)
	TransitionEvent(context.Context, string, int, string, AuditEvent) (model.Event, error)
	SkinExists(context.Context, string) (bool, error)
	CreateSkin(context.Context, Skin, AuditEvent) (Skin, error)
	UpdateSkin(context.Context, string, Skin, AuditEvent) (Skin, error)
	PublishSkinRevision(context.Context, string, string, string, AuditEvent) (SkinRevision, error)
	DisableSkinRevision(context.Context, string, string, AuditEvent) error
	ChangeSkinEntitlement(context.Context, string, string, string, string, string, AuditEvent) (SkinEntitlementEvent, error)
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	Admin       Admin  `json:"admin"`
}

type SessionResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Current   bool      `json:"current"`
}

type accessClaims struct {
	SessionID   string `json:"sid"`
	MFAVerified bool   `json:"mfa"`
	jwt.RegisteredClaims
}

type MFAEnrollmentResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

type MFAConfirmationResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

type suspendUserRequest struct {
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type updateUserDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
	Reason      string `json:"reason"`
	Version     *int   `json:"version"`
}

type MFAChallengeResponse struct {
	MFARequired    bool   `json:"mfa_required"`
	ChallengeToken string `json:"challenge_token"`
}

type LiveRoomClient interface {
	RoomSummary(context.Context, string) (LiveRoomSummary, error)
	HiddenRoomState(context.Context, string) (json.RawMessage, error)
}

type Dependencies struct {
	LiveRooms LiveRoomClient
	Storage   StorageSigner
}

type AdminHandler struct {
	cfg         Config
	store       Store
	storage     StorageSigner
	liveRooms   LiveRoomClient
	uploadMu    sync.Mutex
	uploads     map[string]issuedSkinUpload
	attempts    *loginAttempts
	mfaAttempts *loginAttempts
}

type loginAttempts struct {
	mu      sync.Mutex
	entries map[string][]time.Time
}

func (a *loginAttempts) allow(key string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	cutoff := now.Add(-time.Minute)
	kept := a.entries[key][:0]
	for _, attempt := range a.entries[key] {
		if attempt.After(cutoff) {
			kept = append(kept, attempt)
		}
	}
	if len(kept) >= 5 {
		a.entries[key] = kept
		return false
	}
	a.entries[key] = append(kept, now)
	return true
}

func NewAdminHandler(cfg Config, store Store, deps Dependencies) *AdminHandler {
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	return &AdminHandler{
		cfg:         cfg,
		store:       store,
		storage:     deps.Storage,
		liveRooms:   deps.LiveRooms,
		uploads:     map[string]issuedSkinUpload{},
		attempts:    &loginAttempts{entries: map[string][]time.Time{}},
		mfaAttempts: &loginAttempts{entries: map[string][]time.Time{}},
	}
}

func (h *AdminHandler) Login(c *gin.Context) {
	if !h.attempts.allow(c.ClientIP(), time.Now()) {
		jsonError(c, http.StatusTooManyRequests, "Too many login attempts")
		return
	}
	var req struct {
		Email    string `json:"email" binding:"required,email,max=254"`
		Password string `json:"password" binding:"required,max=1024"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	admin, err := h.store.FindAdminByEmail(c, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil || admin.Status != "active" || bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)) != nil {
		if admin.ID != "" {
			if writeErr := h.store.RecordLoginFailure(c, admin.ID); writeErr != nil {
				jsonError(c, http.StatusInternalServerError, "Internal server error")
				return
			}
		}
		if writeErr := h.store.AppendAudit(c, AuditEvent{AdminID: admin.ID, RequestID: c.GetString("request_id"), Action: "admin.login", Outcome: "denied", IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: time.Now()}); writeErr != nil {
			jsonError(c, http.StatusInternalServerError, "Internal server error")
			return
		}
		jsonError(c, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if err := h.store.RecordLoginSuccess(c, admin.ID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if admin.MFAEnrolled {
		challenge, err := h.challengeToken(admin.ID)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "Internal server error")
			return
		}
		c.JSON(http.StatusAccepted, MFAChallengeResponse{MFARequired: true, ChallengeToken: challenge})
		return
	}
	h.issueSession(c, admin, "admin.login", false)
}

func (h *AdminHandler) EnrollMFA(c *gin.Context) {
	admin := c.MustGet("admin").(Admin)
	if admin.MFAEnrolled && !c.GetBool("mfa_verified") {
		jsonError(c, http.StatusForbidden, "MFA required")
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Seven Spade Admin", AccountName: admin.Email})
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	ciphertext, err := h.encryptSecret(key.Secret())
	if err != nil || h.store.SavePendingMFA(c, admin.ID, ciphertext) != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	c.JSON(http.StatusOK, MFAEnrollmentResponse{Secret: key.Secret(), URI: key.URL()})
}

func (h *AdminHandler) ConfirmMFA(c *gin.Context) {
	admin := c.MustGet("admin").(Admin)
	if !h.mfaAttempts.allow("confirm:"+admin.ID+":"+c.ClientIP(), time.Now()) {
		jsonError(c, http.StatusTooManyRequests, "Too many MFA attempts")
		return
	}
	var req struct {
		Code string `json:"code" binding:"required,len=6"`
	}
	if c.ShouldBindJSON(&req) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	ciphertext, err := h.store.MFASecret(c, admin.ID, false)
	secret, decryptErr := h.decryptSecret(ciphertext)
	if err != nil || decryptErr != nil || !totp.Validate(req.Code, secret) {
		jsonError(c, http.StatusUnauthorized, "Invalid MFA code")
		return
	}
	codes := make([]string, recoveryCodeCount)
	hashes := make([]string, recoveryCodeCount)
	for i := range codes {
		codes[i], err = recoveryCode()
		if err == nil {
			var hash []byte
			hash, err = bcrypt.GenerateFromPassword([]byte(codes[i]), bcrypt.DefaultCost)
			hashes[i] = string(hash)
		}
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "Internal server error")
			return
		}
	}
	if err := h.store.ConfirmMFA(c, admin.ID, hashes); err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	c.JSON(http.StatusOK, MFAConfirmationResponse{RecoveryCodes: codes})
}

func (h *AdminHandler) MFAChallenge(c *gin.Context) {
	var req struct {
		ChallengeToken string `json:"challenge_token" binding:"required"`
		Code           string `json:"code"`
		RecoveryCode   string `json:"recovery_code"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Code == "") == (req.RecoveryCode == "") {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	adminID, err := h.parseChallengeToken(req.ChallengeToken)
	if err == nil && !h.mfaAttempts.allow(adminID+":"+c.ClientIP(), time.Now()) {
		jsonError(c, http.StatusTooManyRequests, "Too many MFA attempts")
		return
	}
	if err != nil {
		jsonError(c, http.StatusUnauthorized, "Invalid MFA challenge")
		return
	}
	admin, err := h.store.FindAdminByID(c, adminID)
	valid := false
	if err == nil && admin.Status == "active" && req.Code != "" {
		ciphertext, secretErr := h.store.MFASecret(c, admin.ID, true)
		secret, decryptErr := h.decryptSecret(ciphertext)
		valid = secretErr == nil && decryptErr == nil && totp.Validate(req.Code, secret)
	} else if err == nil && admin.Status == "active" {
		valid, err = h.store.UseRecoveryCode(c, admin.ID, req.RecoveryCode)
	}
	if err != nil || !valid {
		_ = h.store.AppendAudit(c, AuditEvent{AdminID: adminID, RequestID: c.GetString("request_id"), Action: "admin.mfa.challenge", Outcome: "denied", IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: time.Now()})
		jsonError(c, http.StatusUnauthorized, "Invalid MFA challenge")
		return
	}
	h.issueSession(c, admin, "admin.mfa.challenge", true)
}

func (h *AdminHandler) Refresh(c *gin.Context) {
	if !validCSRF(c) {
		jsonError(c, http.StatusForbidden, "Invalid CSRF token")
		return
	}
	raw, err := c.Cookie(refreshCookieName)
	if err != nil || raw == "" {
		jsonError(c, http.StatusUnauthorized, "Invalid session")
		return
	}
	newRaw, err := randomToken()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	session, err := h.store.RotateSession(c, hashToken(raw), hashToken(newRaw), uuid.NewString(), time.Now().Add(h.cfg.RefreshTTL))
	if err != nil {
		h.clearCookie(c)
		jsonError(c, http.StatusUnauthorized, "Invalid session")
		return
	}
	admin, err := h.store.FindAdminByID(c, session.AdminID)
	if err != nil || admin.Status != "active" {
		_ = h.store.RevokeSession(c, hashToken(newRaw))
		h.clearCookie(c)
		jsonError(c, http.StatusUnauthorized, "Invalid session")
		return
	}
	access, err := h.accessToken(admin.ID, session.ID, session.MFAVerified)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := h.store.AppendAudit(c, AuditEvent{AdminID: admin.ID, SessionID: session.ID, RequestID: c.GetString("request_id"), Action: "admin.refresh", Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: time.Now()}); err != nil {
		_ = h.store.RevokeSession(c, hashToken(newRaw))
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.setCookie(c, newRaw)
	c.JSON(http.StatusOK, AuthResponse{AccessToken: access, Admin: admin})
}

func (h *AdminHandler) Logout(c *gin.Context) {
	if !validCSRF(c) {
		jsonError(c, http.StatusForbidden, "Invalid CSRF token")
		return
	}
	if raw, err := c.Cookie(refreshCookieName); err == nil {
		if err := h.store.RevokeSession(c, hashToken(raw)); err != nil {
			jsonError(c, http.StatusInternalServerError, "Internal server error")
			return
		}
	}
	if err := h.store.AppendAudit(c, AuditEvent{RequestID: c.GetString("request_id"), Action: "admin.logout", Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: time.Now()}); err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.clearCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) issueSession(c *gin.Context, admin Admin, action string, mfaVerified bool) {
	raw, err := randomToken()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	now := time.Now()
	session := Session{ID: uuid.NewString(), FamilyID: uuid.NewString(), AdminID: admin.ID, TokenHash: hashToken(raw), ExpiresAt: now.Add(h.cfg.RefreshTTL), CreatedAt: now, IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), MFAVerified: mfaVerified}
	if err := h.store.CreateSession(c, session); err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	access, err := h.accessToken(admin.ID, session.ID, session.MFAVerified)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := h.store.AppendAudit(c, AuditEvent{AdminID: admin.ID, SessionID: session.ID, RequestID: c.GetString("request_id"), Action: action, Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: time.Now()}); err != nil {
		_ = h.store.RevokeSession(c, session.TokenHash)
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.setCookie(c, raw)
	c.JSON(http.StatusOK, AuthResponse{AccessToken: access, Admin: admin})
}

func (h *AdminHandler) accessToken(adminID, sessionID string, mfaVerified bool) (string, error) {
	now := time.Now()
	claims := accessClaims{SessionID: sessionID, MFAVerified: mfaVerified, RegisteredClaims: jwt.RegisteredClaims{Subject: adminID, Issuer: "seven-spade-admin", Audience: jwt.ClaimStrings{"admin-api"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.AccessTTL))}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.JWTSecret))
}

func (h *AdminHandler) RequireAuth(c *gin.Context) {
	raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(h.cfg.JWTSecret), nil
	}, jwt.WithIssuer("seven-spade-admin"), jwt.WithAudience("admin-api"))
	if err != nil || !token.Valid {
		jsonError(c, http.StatusUnauthorized, "Authentication required")
		c.Abort()
		return
	}
	admin, err := h.store.FindAdminByID(c, claims.Subject)
	session, sessionErr := h.store.SessionActive(c, claims.SessionID, claims.Subject)
	if err != nil || sessionErr != nil || session.MFAVerified != claims.MFAVerified || admin.Status != "active" {
		jsonError(c, http.StatusUnauthorized, "Authentication required")
		c.Abort()
		return
	}
	c.Set("admin", admin)
	c.Set("session_id", claims.SessionID)
	c.Set("mfa_verified", claims.MFAVerified)
	isMFASetup := c.Request.URL.Path == "/auth/mfa/enroll" || c.Request.URL.Path == "/auth/mfa/confirm"
	if h.cfg.Environment == "production" && !isMFASetup && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions && !claims.MFAVerified {
		jsonError(c, http.StatusForbidden, "MFA required")
		c.Abort()
		return
	}
	c.Next()
}

func (h *AdminHandler) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		admin := c.MustGet("admin").(Admin)
		for _, value := range admin.Permissions {
			if value == permission {
				c.Next()
				return
			}
		}
		event := h.requestAudit(c, admin.ID, "permission.denied", "http_route", c.FullPath(), "rejected")
		event.Metadata = []byte(`{"permission":"` + permission + `"}`)
		if action, resourceType := auditActionForRequest(c); action != "" {
			event.Action = action
			event.ResourceType = resourceType
			event.ResourceID = c.Param("id")
		}
		if err := h.store.AppendAudit(c, event); err != nil {
			jsonError(c, http.StatusServiceUnavailable, "Audit trail unavailable")
			c.Abort()
			return
		}
		jsonError(c, http.StatusForbidden, "Permission denied")
		c.Abort()
	}
}

func hasPermission(permissions []string, permission string) bool {
	for _, value := range permissions {
		if value == permission {
			return true
		}
	}
	return false
}

func (h *AdminHandler) requestAudit(c *gin.Context, adminID, action, resourceType, resourceID, outcome string) AuditEvent {
	return AuditEvent{AdminID: adminID, SessionID: c.GetString("session_id"), RequestID: c.GetString("request_id"), Action: action, ResourceType: resourceType, ResourceID: resourceID, Outcome: outcome, IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: time.Now()}
}

func auditActionForRequest(c *gin.Context) (string, string) {
	switch c.FullPath() {
	case "/admins/:id/status":
		return "admin.status.update", "admin_user"
	case "/admins/:id/roles":
		return "admin.roles.update", "admin_user"
	case "/roles/:id/permissions":
		return "admin.role_permissions.update", "admin_role"
	case "/admins/invite":
		return "admin.invite", "admin_invitation"
	default:
		return "", ""
	}
}

func (h *AdminHandler) Me(c *gin.Context) { c.JSON(http.StatusOK, c.MustGet("admin")) }

func (h *AdminHandler) ListSessions(c *gin.Context) {
	admin := c.MustGet("admin").(Admin)
	currentID := c.GetString("session_id")
	sessions, err := h.store.ListSessions(c, admin.ID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load sessions")
		return
	}
	result := make([]SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, SessionResponse{ID: session.ID, CreatedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt, IPAddress: session.IPAddress, UserAgent: session.UserAgent, Current: session.ID == currentID})
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) RevokeSession(c *gin.Context) {
	admin := c.MustGet("admin").(Admin)
	actorSessionID := c.GetString("session_id")
	targetID := c.Param("id")
	if _, err := uuid.Parse(targetID); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid session ID")
		return
	}
	event := h.sessionAudit(c, admin.ID, actorSessionID, "admin.session.revoke", targetID)
	revoked, err := h.store.RevokeSessionByID(c, admin.ID, targetID, event)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to revoke session")
		return
	}
	if !revoked {
		jsonError(c, http.StatusNotFound, "Session not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) RevokeOtherSessions(c *gin.Context) {
	admin := c.MustGet("admin").(Admin)
	currentID := c.GetString("session_id")
	event := h.sessionAudit(c, admin.ID, currentID, "admin.sessions.revoke_others", admin.ID)
	if err := h.store.RevokeOtherSessions(c, admin.ID, currentID, event); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to revoke sessions")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) sessionAudit(c *gin.Context, adminID, sessionID, action, resourceID string) AuditEvent {
	return AuditEvent{AdminID: adminID, SessionID: sessionID, RequestID: c.GetString("request_id"), Action: action, ResourceType: "admin_session", ResourceID: resourceID, Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: time.Now()}
}

type InviteAdminRequest struct {
	Email  string `json:"email" binding:"required,email,max=254"`
	RoleID string `json:"role_id" binding:"required"`
}

type InviteAdminResponse struct {
	Invitation Invitation `json:"invitation"`
	Token      string     `json:"token"`
}

type AcceptInviteRequest struct {
	Token       string `json:"token" binding:"required"`
	DisplayName string `json:"display_name" binding:"required,min=2,max=64"`
	Password    string `json:"password" binding:"required,min=8,max=1024"`
}

type ReissueInviteResponse struct {
	Invitation Invitation `json:"invitation"`
	Token      string     `json:"token"`
}

type InvitationPreview struct {
	Email     string    `json:"email"`
	RoleName  string    `json:"role_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

type SetAdminStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active disabled"`
	Reason string `json:"reason" binding:"omitempty,min=3,max=500"`
}

type SetAdminRolesRequest struct {
	RoleIDs []string `json:"role_ids" binding:"required"`
}

type UpdateRolePermissionsRequest struct {
	Permissions []string `json:"permissions" binding:"required"`
}

func (h *AdminHandler) ListAdmins(c *gin.Context) {
	admins, err := h.store.ListAdmins(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load administrators")
		return
	}
	c.JSON(http.StatusOK, admins)
}

func (h *AdminHandler) InviteAdmin(c *gin.Context) {
	actor := c.MustGet("admin").(Admin)
	var req InviteAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	rawToken, err := randomToken()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	invitation := Invitation{
		ID:        uuid.NewString(),
		Email:     email,
		RoleID:    req.RoleID,
		InvitedBy: actor.ID,
		ExpiresAt: time.Now().Add(48 * time.Hour),
		CreatedAt: time.Now(),
	}
	tokenHash := hashToken(rawToken)
	invitation.TokenHash = tokenHash
	event := h.requestAudit(c, actor.ID, "admin.invite", "admin_invitation", invitation.ID, "success")
	event.ID = uuid.NewString()
	event.AfterState = []byte(`{"email":"` + invitation.Email + `","role_id":"` + invitation.RoleID + `"}`)
	if err := h.store.CreateInvitation(c, invitation, event); err != nil {
		if errors.Is(err, ErrConflict) {
			jsonError(c, http.StatusConflict, "Administrator or active invitation already exists for this email")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to create invitation")
		return
	}
	c.Header("Audit-Event-ID", event.ID)
	c.JSON(http.StatusCreated, InviteAdminResponse{
		Invitation: invitation,
		Token:      rawToken,
	})
}

func (h *AdminHandler) GetInvitation(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if c.ShouldBindJSON(&req) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	invitation, err := h.store.FindInvitationByTokenHash(c, hashToken(req.Token))
	if err != nil {
		jsonError(c, http.StatusNotFound, "Invalid or expired invitation token")
		return
	}
	c.JSON(http.StatusOK, InvitationPreview{Email: invitation.Email, RoleName: invitation.RoleName, ExpiresAt: invitation.ExpiresAt})
}

func (h *AdminHandler) ListInvitations(c *gin.Context) {
	invitations, err := h.store.ListInvitations(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load invitations")
		return
	}
	c.JSON(http.StatusOK, invitations)
}

func (h *AdminHandler) RevokeInvitation(c *gin.Context) {
	actor := c.MustGet("admin").(Admin)
	event := h.requestAudit(c, actor.ID, "admin.invite.revoke", "admin_invitation", c.Param("id"), "success")
	if err := h.store.RevokeInvitation(c, c.Param("id"), event); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "Active invitation not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to revoke invitation")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) ReissueInvitation(c *gin.Context) {
	actor := c.MustGet("admin").(Admin)
	rawToken, err := randomToken()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	expiresAt := time.Now().Add(48 * time.Hour)
	event := h.requestAudit(c, actor.ID, "admin.invite.reissue", "admin_invitation", c.Param("id"), "success")
	invitation, err := h.store.ReissueInvitation(c, c.Param("id"), hashToken(rawToken), expiresAt, event)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "Active invitation not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to reissue invitation")
		return
	}
	c.JSON(http.StatusOK, ReissueInviteResponse{Invitation: invitation, Token: rawToken})
}

func (h *AdminHandler) AcceptInvite(c *gin.Context) {
	var req AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len([]byte(req.Password)) > 72 {
		jsonError(c, http.StatusBadRequest, "Password must be 72 bytes or fewer")
		return
	}
	tokenHash := hashToken(req.Token)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	event := h.requestAudit(c, "", "admin.invite.accept", "admin_user", "", "success")
	event.ID = uuid.NewString()
	admin, err := h.store.AcceptInvitation(c, tokenHash, strings.TrimSpace(req.DisplayName), string(passwordHash), event)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "Invalid or expired invitation token")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to accept invitation")
		return
	}
	c.Header("Audit-Event-ID", event.ID)
	c.JSON(http.StatusOK, admin)
}

func (h *AdminHandler) SetAdminStatus(c *gin.Context) {
	actor := c.MustGet("admin").(Admin)
	targetID := c.Param("id")
	if targetID == "" {
		jsonError(c, http.StatusBadRequest, "Invalid administrator ID")
		return
	}
	var req SetAdminStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	event := AuditEvent{
		ID:           uuid.NewString(),
		AdminID:      actor.ID,
		SessionID:    c.GetString("session_id"),
		RequestID:    c.GetString("request_id"),
		Action:       "admin.status.update",
		ResourceType: "admin_user",
		ResourceID:   targetID,
		Reason:       strings.TrimSpace(req.Reason),
		Outcome:      "success",
		AfterState:   []byte(`{"status":"` + req.Status + `"}`),
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		OccurredAt:   time.Now(),
	}
	if err := h.store.SetAdminStatus(c, targetID, req.Status, event); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "Administrator not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to update administrator status")
		return
	}
	if req.Status == "disabled" {
		_ = h.store.RevokeAdminSessions(c, targetID)
	}
	c.Header("Audit-Event-ID", event.ID)
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) SetAdminRoles(c *gin.Context) {
	actor := c.MustGet("admin").(Admin)
	targetID := c.Param("id")
	if targetID == "" {
		jsonError(c, http.StatusBadRequest, "Invalid administrator ID")
		return
	}
	var req SetAdminRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	event := AuditEvent{
		AdminID:      actor.ID,
		SessionID:    c.GetString("session_id"),
		RequestID:    c.GetString("request_id"),
		Action:       "admin.roles.update",
		ResourceType: "admin_user",
		ResourceID:   targetID,
		Outcome:      "success",
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		OccurredAt:   time.Now(),
	}
	if err := h.store.SetAdminRoles(c, targetID, req.RoleIDs, event); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "Administrator or role not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to update administrator roles")
		return
	}
	_ = h.store.RevokeAdminSessions(c, targetID)
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) ListRoles(c *gin.Context) {
	roles, err := h.store.ListRoles(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load roles")
		return
	}
	c.JSON(http.StatusOK, roles)
}

func (h *AdminHandler) ListPermissions(c *gin.Context) {
	permissions, err := h.store.ListPermissions(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load permissions")
		return
	}
	c.JSON(http.StatusOK, permissions)
}

func (h *AdminHandler) UpdateRolePermissions(c *gin.Context) {
	actor := c.MustGet("admin").(Admin)
	roleID := c.Param("id")
	if roleID == "" {
		jsonError(c, http.StatusBadRequest, "Invalid role ID")
		return
	}
	var req UpdateRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	event := AuditEvent{
		AdminID:      actor.ID,
		SessionID:    c.GetString("session_id"),
		RequestID:    c.GetString("request_id"),
		Action:       "admin.role_permissions.update",
		ResourceType: "admin_role",
		ResourceID:   roleID,
		Outcome:      "success",
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		OccurredAt:   time.Now(),
	}
	if err := h.store.UpdateRolePermissions(c, roleID, req.Permissions, event); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "Role not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to update role permissions")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) ListAuditEvents(c *gin.Context) {
	filter, ok := auditFilterFromRequest(c, false)
	if !ok {
		return
	}
	result, err := h.store.ListAuditEvents(c, filter)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load audit events")
		return
	}
	actor := c.MustGet("admin").(Admin)
	if err := h.store.AppendAudit(c, h.requestAudit(c, actor.ID, "audit.events.read", "admin_audit_event", "", "success")); err != nil {
		jsonError(c, http.StatusServiceUnavailable, "Audit trail unavailable")
		return
	}
	c.JSON(http.StatusOK, result)
}

func auditFilterFromRequest(c *gin.Context, exporting bool) (AuditFilter, bool) {
	filter := AuditFilter{ID: c.Query("id"), ActorID: c.Query("actor_id"), Action: c.Query("action"), ResourceType: c.Query("resource_type"), ResourceID: c.Query("resource_id"), Outcome: c.Query("outcome"), Limit: 50}
	for name, value := range map[string]string{"action": filter.Action, "resource_type": filter.ResourceType, "resource_id": filter.ResourceID} {
		if len(value) > 255 {
			jsonError(c, http.StatusBadRequest, "Invalid "+name+" filter")
			return AuditFilter{}, false
		}
	}
	if filter.Outcome != "" && filter.Outcome != "success" && filter.Outcome != "rejected" && filter.Outcome != "failed" {
		jsonError(c, http.StatusBadRequest, "Invalid outcome filter")
		return AuditFilter{}, false
	}
	if filter.ID != "" {
		if _, err := uuid.Parse(filter.ID); err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid audit event ID")
			return AuditFilter{}, false
		}
	}
	if filter.ActorID != "" {
		if _, err := uuid.Parse(filter.ActorID); err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid actor ID")
			return AuditFilter{}, false
		}
	}
	if value := c.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			jsonError(c, http.StatusBadRequest, "Invalid limit")
			return AuditFilter{}, false
		}
		filter.Limit = limit
	}
	if value := c.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			jsonError(c, http.StatusBadRequest, "Invalid offset")
			return AuditFilter{}, false
		}
		filter.Offset = offset
	}
	for value, destination := range map[string]**time.Time{"from": &filter.From, "to": &filter.To} {
		if raw := c.Query(value); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				jsonError(c, http.StatusBadRequest, "Invalid "+value+" timestamp")
				return AuditFilter{}, false
			}
			*destination = &parsed
		}
	}
	if filter.From != nil && filter.To != nil && !filter.From.Before(*filter.To) {
		jsonError(c, http.StatusBadRequest, "Invalid time range")
		return AuditFilter{}, false
	}
	if exporting && (filter.From == nil || filter.To == nil || filter.To.Sub(*filter.From) > 31*24*time.Hour) {
		jsonError(c, http.StatusBadRequest, "Export requires a time range of at most 31 days")
		return AuditFilter{}, false
	}
	return filter, true
}

func (h *AdminHandler) ExportAuditEvents(c *gin.Context) {
	actor := c.MustGet("admin").(Admin)
	filter, ok := auditFilterFromRequest(c, true)
	if !ok {
		if err := h.store.AppendAudit(c, h.requestAudit(c, actor.ID, "audit.events.export", "admin_audit_event", "", "rejected")); err != nil {
			jsonError(c, http.StatusServiceUnavailable, "Audit trail unavailable")
		}
		return
	}
	filter.Limit = 100
	filter.Offset = 0
	rows := make([]AuditEvent, 0, 100)
	for len(rows) < auditExportLimit {
		page, err := h.store.ListAuditEvents(c, filter)
		if err != nil {
			if auditErr := h.store.AppendAudit(c, h.requestAudit(c, actor.ID, "audit.events.export", "admin_audit_event", "", "failed")); auditErr != nil {
				jsonError(c, http.StatusServiceUnavailable, "Audit trail unavailable")
				return
			}
			jsonError(c, http.StatusInternalServerError, "Failed to export audit events")
			return
		}
		rows = append(rows, page.Events...)
		if len(page.Events) < filter.Limit {
			break
		}
		filter.Offset += len(page.Events)
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.Write([]string{"id", "actor_id", "request_id", "action", "resource_type", "resource_id", "outcome", "occurred_at"}); err == nil {
		for _, row := range rows {
			if err = writer.Write([]string{csvSafe(row.ID), csvSafe(row.AdminID), csvSafe(row.RequestID), csvSafe(row.Action), csvSafe(row.ResourceType), csvSafe(row.ResourceID), csvSafe(row.Outcome), row.OccurredAt.Format(time.RFC3339Nano)}); err != nil {
				break
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		if auditErr := h.store.AppendAudit(c, h.requestAudit(c, actor.ID, "audit.events.export", "admin_audit_event", "", "failed")); auditErr != nil {
			jsonError(c, http.StatusServiceUnavailable, "Audit trail unavailable")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to encode audit export")
		return
	}
	event := h.requestAudit(c, actor.ID, "audit.events.export", "admin_audit_event", "", "success")
	event.Metadata = []byte(fmt.Sprintf(`{"exported_rows":%d,"truncated":%t}`, len(rows), len(rows) == auditExportLimit))
	if err := h.store.AppendAudit(c, event); err != nil {
		jsonError(c, http.StatusServiceUnavailable, "Audit trail unavailable")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="audit-events.csv"`)
	_, _ = c.Writer.Write(output.Bytes())
}

func csvSafe(value string) string {
	if value != "" && strings.ContainsRune("=+-@", rune(value[0])) {
		return "'" + value
	}
	return value
}

func (h *AdminHandler) Dashboard(c *gin.Context) {
	result, err := h.store.Dashboard(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load dashboard")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) SearchUsers(c *gin.Context) {
	limit, offset, ok := pageFromRequest(c)
	if !ok {
		return
	}
	sensitive := h.hasPermission(c, "users.sensitive.read")
	result, err := h.store.SearchUsers(c, strings.TrimSpace(c.Query("query")), limit, offset, sensitive)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to search users")
		return
	}
	if sensitive && c.Query("query") != "" {
		actor := c.MustGet("admin").(Admin)
		if err := h.store.AppendAudit(c, h.requestAudit(c, actor.ID, "users.search.sensitive", "user", "", "success")); err != nil {
			jsonError(c, http.StatusServiceUnavailable, "Audit trail unavailable")
			return
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")
	if _, err := uuid.Parse(userID); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	result, err := h.store.GetUser(c, userID, h.hasPermission(c, "users.sensitive.read"))
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load user")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) SuspendUser(c *gin.Context) {
	userID := c.Param("id")
	if _, err := uuid.Parse(userID); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	var req suspendUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, http.StatusBadRequest, "Suspension reason is required")
		return
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		jsonError(c, http.StatusBadRequest, "Suspension expiry must be in the future")
		return
	}
	actor := c.MustGet("admin").(Admin)
	suspension := Suspension{Reason: strings.TrimSpace(req.Reason), ExpiresAt: req.ExpiresAt}
	event := h.requestAudit(c, actor.ID, "user.suspend", "user", userID, "success")
	event.ID = uuid.NewString()
	event.Reason = suspension.Reason
	if err := h.store.SuspendUser(c, userID, suspension, event); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "User not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to suspend user")
		return
	}
	c.JSON(http.StatusOK, gin.H{"suspension": suspension, "audit_action": event.Action, "audit_event_id": event.ID})
}

func (h *AdminHandler) ReinstateUser(c *gin.Context) {
	userID := c.Param("id")
	if _, err := uuid.Parse(userID); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	actor := c.MustGet("admin").(Admin)
	event := h.requestAudit(c, actor.ID, "user.reinstate", "user", userID, "success")
	event.ID = uuid.NewString()
	if err := h.store.ReinstateUser(c, userID, event); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "User not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to reinstate user")
		return
	}
	c.JSON(http.StatusOK, gin.H{"audit_action": event.Action, "audit_event_id": event.ID})
}

func (h *AdminHandler) UpdateUserDisplayName(c *gin.Context) {
	userID := c.Param("id")
	if _, err := uuid.Parse(userID); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	var req updateUserDisplayNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request")
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	reason := strings.TrimSpace(req.Reason)
	if displayName == "" || len([]rune(displayName)) > 50 {
		jsonError(c, http.StatusBadRequest, "Display name must be between 1 and 50 characters")
		return
	}
	if reason == "" {
		jsonError(c, http.StatusBadRequest, "Moderation reason is required")
		return
	}
	if req.Version == nil || *req.Version < 1 {
		jsonError(c, http.StatusBadRequest, "Expected resource version is required")
		return
	}
	actor := c.MustGet("admin").(Admin)
	event := h.requestAudit(c, actor.ID, "user.display_name.update", "user", userID, "success")
	event.ID = uuid.NewString()
	event.Reason = reason
	user, err := h.store.UpdateUserDisplayName(c, userID, displayName, *req.Version, event)
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "User not found")
		return
	}
	if errors.Is(err, ErrConflict) {
		jsonError(c, http.StatusConflict, "User changed since it was loaded")
		return
	}
	if errors.Is(err, ErrUserActive) {
		jsonError(c, http.StatusConflict, "Display name cannot be changed while the player is in an active room")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update display name")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user, "audit_action": event.Action, "audit_event_id": event.ID})
}

func pageFromRequest(c *gin.Context) (int, int, bool) {
	limit, offset := 50, 0
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			jsonError(c, http.StatusBadRequest, "Invalid limit")
			return 0, 0, false
		}
		limit = parsed
	}
	if value := c.Query("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > 100000 {
			jsonError(c, http.StatusBadRequest, "Invalid offset")
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
}

func (h *AdminHandler) hasPermission(c *gin.Context, permission string) bool {
	for _, value := range c.MustGet("admin").(Admin).Permissions {
		if value == permission {
			return true
		}
	}
	return false
}

func (h *AdminHandler) setCookie(c *gin.Context, token string) {
	csrf := hashToken(token)
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, token, int(h.cfg.RefreshTTL.Seconds()), "/auth", "", h.cfg.SecureCookies, true)
	c.SetCookie(csrfCookieName, csrf, int(h.cfg.RefreshTTL.Seconds()), "/", "", h.cfg.SecureCookies, false)
}
func (h *AdminHandler) clearCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, "/auth", "", h.cfg.SecureCookies, true)
	c.SetCookie(csrfCookieName, "", -1, "/", "", h.cfg.SecureCookies, false)
}
func validCSRF(c *gin.Context) bool {
	cookie, err := c.Cookie(csrfCookieName)
	header := c.GetHeader("X-CSRF-Token")
	return err == nil && cookie != "" && header != "" && subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) == 1
}
func (h *AdminHandler) challengeToken(adminID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{Subject: adminID, Issuer: "seven-spade-admin", Audience: jwt.ClaimStrings{"admin-mfa-challenge"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute))}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.JWTSecret))
}

func (h *AdminHandler) parseChallengeToken(raw string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(h.cfg.JWTSecret), nil
	}, jwt.WithIssuer("seven-spade-admin"), jwt.WithAudience("admin-mfa-challenge"))
	if err != nil || !token.Valid || claims.Subject == "" {
		return "", errors.New("invalid challenge")
	}
	return claims.Subject, nil
}

func (h *AdminHandler) encryptionKey() []byte {
	sum := sha256.Sum256([]byte(h.cfg.MFAEncryptionKey))
	return sum[:]
}

func (h *AdminHandler) encryptSecret(secret string) ([]byte, error) {
	block, err := aes.NewCipher(h.encryptionKey())
	if err != nil {
		return nil, err
	}
	var aead cipher.AEAD
	if aead, err = cipher.NewGCM(block); err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(secret), nil), nil
}

func (h *AdminHandler) decryptSecret(ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(h.encryptionKey())
	if err != nil {
		return "", err
	}
	var aead cipher.AEAD
	if aead, err = cipher.NewGCM(block); err != nil {
		return "", err
	}
	if len(ciphertext) < aead.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}
	plain, err := aead.Open(nil, ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():], nil)
	return string(plain), err
}

func recoveryCode() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(base64.RawURLEncoding.EncodeToString(b)), nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func jsonError(c *gin.Context, status int, message string) { c.JSON(status, gin.H{"error": message}) }

// MemoryStore is the behaviorally equivalent test adapter for the Store seam.
type MemoryStore struct {
	mu                           sync.Mutex
	admins                       map[string]Admin
	roles                        map[string]Role
	permissions                  []Permission
	invites                      map[string]Invitation // keyed by tokenHash
	sessions                     map[string]Session
	audits                       []AuditEvent
	mfa                          map[string][]byte
	verified                     map[string]bool
	recovery                     map[string][]string
	users                        map[string]UserDetail
	rooms                        map[string]RoomDetail
	games                        map[string]GameDetail
	skins                        map[string]Skin
	entitlements                 map[string]string
	entitlementEvents            []SkinEntitlementEvent
	achievements                 map[string]model.Achievement
	achievementEntitlements      map[string]bool
	achievementEntitlementEvents []model.AchievementEntitlementEvent
	achievementIdempotency       map[string]model.AchievementEntitlementEvent
	events                       map[string]model.Event
	featureSettings              map[string]bool
}

func NewMemoryStore(admins ...Admin) *MemoryStore {
	s := &MemoryStore{
		admins:                  map[string]Admin{},
		roles:                   map[string]Role{},
		invites:                 map[string]Invitation{},
		sessions:                map[string]Session{},
		mfa:                     map[string][]byte{},
		verified:                map[string]bool{},
		recovery:                map[string][]string{},
		users:                   map[string]UserDetail{},
		rooms:                   map[string]RoomDetail{},
		games:                   map[string]GameDetail{},
		skins:                   map[string]Skin{},
		entitlements:            map[string]string{},
		achievements:            map[string]model.Achievement{},
		achievementEntitlements: map[string]bool{},
		achievementIdempotency:  map[string]model.AchievementEntitlementEvent{},
		events:                  map[string]model.Event{},
		featureSettings: map[string]bool{
			"daily_login": true, "new_registrations": true, "guest_access": true,
			"room_creation": true, "quick_play": true, "new_game_starts": true,
			"spectator_access": true, "emotes": true,
		},
		permissions: []Permission{
			{Name: "dashboard.read", Description: "View the admin operations dashboard"},
			{Name: "users.read", Description: "View users"},
			{Name: "users.sensitive.read", Description: "View user email addresses"},
			{Name: "users.moderate", Description: "Moderate users"},
			{Name: "users.economy.adjust", Description: "Adjust rating and XP"},
			{Name: "rooms.read", Description: "View rooms"},
			{Name: "rooms.inspect_hidden", Description: "Inspect hidden live game state"},
			{Name: "rooms.terminate", Description: "Terminate rooms"},
			{Name: "games.read", Description: "View games"},
			{Name: "games.annotate", Description: "Flag games and add administrative notes"},
			{Name: "games.invalidate", Description: "Invalidate games"},
			{Name: "seasons.read", Description: "View seasons"},
			{Name: "seasons.manage", Description: "Manage seasons"},
			{Name: "events.read", Description: "View events"},
			{Name: "events.manage", Description: "Manage events"},
			{Name: "achievements.read", Description: "View achievements"},
			{Name: "achievements.manage", Description: "Manage achievements"},
			{Name: "achievements.entitlements", Description: "Grant and revoke exceptional achievement entitlements"},
			{Name: "skins.read", Description: "View skins"},
			{Name: "skins.manage", Description: "Manage skins"},
			{Name: "skins.entitlements", Description: "Correct skin entitlements"},
			{Name: "admins.read", Description: "View administrators"},
			{Name: "admins.manage", Description: "Manage administrator identities and roles"},
			{Name: "audit.read", Description: "View administrator audit events"},
			{Name: "audit.export", Description: "Export redacted administrator audit events"},
			{Name: "settings.read", Description: "View application settings"},
			{Name: "settings.write", Description: "Manage application settings"},
		},
	}
	s.roles["role-super"] = Role{ID: "role-super", Name: "super_admin", Description: "Full administrator access", Permissions: []string{"dashboard.read", "users.read", "users.sensitive.read", "users.moderate", "users.economy.adjust", "rooms.read", "rooms.inspect_hidden", "rooms.terminate", "games.read", "games.annotate", "games.invalidate", "seasons.read", "seasons.manage", "events.read", "events.manage", "achievements.read", "achievements.manage", "skins.read", "skins.manage", "admins.read", "admins.manage", "audit.read", "audit.export", "settings.read", "settings.write"}}
	s.roles["role-viewer"] = Role{ID: "role-viewer", Name: "viewer", Description: "Read-only operational access", Permissions: []string{"dashboard.read"}}
	s.roles["role-moderator"] = Role{ID: "role-moderator", Name: "moderator", Description: "User and room moderation access", Permissions: []string{"dashboard.read", "users.read", "users.moderate", "rooms.read", "rooms.terminate", "games.read", "audit.read"}}
	s.roles["role-operator"] = Role{ID: "role-operator", Name: "operator", Description: "Content and operational management access", Permissions: []string{"dashboard.read", "users.read", "users.moderate", "rooms.read", "rooms.terminate", "games.read", "games.annotate", "seasons.read", "seasons.manage", "events.read", "events.manage", "achievements.read", "achievements.manage", "skins.read", "skins.manage", "admins.read", "audit.read"}}

	for _, a := range admins {
		s.admins[a.ID] = a
	}
	return s
}

func (s *MemoryStore) ListAdmins(_ context.Context) ([]Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	admins := make([]Admin, 0, len(s.admins))
	for _, a := range s.admins {
		admins = append(admins, a)
	}
	return admins, nil
}

func (s *MemoryStore) CreateInvitation(_ context.Context, inv Invitation, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.admins {
		if strings.EqualFold(a.Email, inv.Email) {
			return ErrConflict
		}
	}
	if role, ok := s.roles[inv.RoleID]; ok {
		inv.RoleName = role.Name
	}
	for _, existing := range s.invites {
		if strings.EqualFold(existing.Email, inv.Email) && existing.AcceptedAt == nil && existing.RevokedAt == nil && time.Now().Before(existing.ExpiresAt) {
			return ErrConflict
		}
	}
	s.invites[inv.TokenHash] = inv
	s.audits = append(s.audits, event)
	return nil
}

func (s *MemoryStore) FindInvitationByTokenHash(_ context.Context, tokenHash string) (Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[tokenHash]
	if !ok || inv.AcceptedAt != nil || inv.RevokedAt != nil || time.Now().After(inv.ExpiresAt) {
		return Invitation{}, ErrNotFound
	}
	return inv, nil
}

func (s *MemoryStore) ListInvitations(_ context.Context) ([]Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	invitations := make([]Invitation, 0, len(s.invites))
	for _, invitation := range s.invites {
		invitations = append(invitations, invitation)
	}
	return invitations, nil
}

func (s *MemoryStore) RevokeInvitation(_ context.Context, id string, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, invitation := range s.invites {
		if invitation.ID == id && invitation.AcceptedAt == nil && invitation.RevokedAt == nil && time.Now().Before(invitation.ExpiresAt) {
			now := time.Now()
			invitation.RevokedAt = &now
			s.invites[hash] = invitation
			s.audits = append(s.audits, event)
			return nil
		}
	}
	return ErrNotFound
}

func (s *MemoryStore) ReissueInvitation(_ context.Context, id, tokenHash string, expiresAt time.Time, event AuditEvent) (Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, invitation := range s.invites {
		if invitation.ID == id && invitation.AcceptedAt == nil && invitation.RevokedAt == nil {
			delete(s.invites, hash)
			invitation.TokenHash = tokenHash
			invitation.ExpiresAt = expiresAt
			s.invites[tokenHash] = invitation
			s.audits = append(s.audits, event)
			return invitation, nil
		}
	}
	return Invitation{}, ErrNotFound
}

func (s *MemoryStore) AcceptInvitation(_ context.Context, tokenHash, displayName, passwordHash string, event AuditEvent) (Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[tokenHash]
	if !ok || inv.AcceptedAt != nil || inv.RevokedAt != nil || time.Now().After(inv.ExpiresAt) {
		return Admin{}, ErrNotFound
	}
	now := time.Now()
	inv.AcceptedAt = &now
	s.invites[tokenHash] = inv
	role := s.roles[inv.RoleID]
	admin := Admin{
		ID:           uuid.NewString(),
		Email:        inv.Email,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		Status:       "active",
		Roles:        []Role{role},
		Permissions:  role.Permissions,
		CreatedAt:    now,
	}
	s.admins[admin.ID] = admin
	event.AdminID, event.ResourceID = admin.ID, admin.ID
	event.AfterState = []byte(`{"status":"active"}`)
	s.audits = append(s.audits, event)
	return admin, nil
}

func (s *MemoryStore) SetAdminStatus(_ context.Context, id, status string, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.admins[id]
	if !ok {
		return ErrNotFound
	}
	a.Status = status
	s.admins[id] = a
	s.audits = append(s.audits, event)
	return nil
}

func (s *MemoryStore) SetAdminRoles(_ context.Context, id string, roleIDs []string, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.admins[id]
	if !ok {
		return ErrNotFound
	}
	var roles []Role
	var permissions []string
	permSet := map[string]bool{}
	for _, rid := range roleIDs {
		r, ok := s.roles[rid]
		if !ok {
			return ErrNotFound
		}
		roles = append(roles, r)
		for _, p := range r.Permissions {
			if !permSet[p] {
				permSet[p] = true
				permissions = append(permissions, p)
			}
		}
	}
	a.Roles = roles
	a.Permissions = permissions
	s.admins[id] = a
	s.audits = append(s.audits, event)
	return nil
}

func (s *MemoryStore) ListRoles(_ context.Context) ([]Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	roles := make([]Role, 0, len(s.roles))
	for _, r := range s.roles {
		roles = append(roles, r)
	}
	return roles, nil
}

func (s *MemoryStore) ListPermissions(_ context.Context) ([]Permission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Permission(nil), s.permissions...), nil
}

func (s *MemoryStore) UpdateRolePermissions(_ context.Context, roleID string, perms []string, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.roles[roleID]
	if !ok {
		return ErrNotFound
	}
	r.Permissions = perms
	s.roles[roleID] = r
	now := time.Now()
	for id, admin := range s.admins {
		var newPerms []string
		permSet := map[string]bool{}
		hasRole := false
		for _, ar := range admin.Roles {
			if ar.ID == roleID {
				hasRole = true
			}
			curRole := s.roles[ar.ID]
			for _, p := range curRole.Permissions {
				if !permSet[p] {
					permSet[p] = true
					newPerms = append(newPerms, p)
				}
			}
		}
		if len(admin.Roles) > 0 {
			admin.Permissions = newPerms
			s.admins[id] = admin
		}
		if hasRole {
			for hash, session := range s.sessions {
				if session.AdminID == id && session.RevokedAt == nil {
					session.RevokedAt = &now
					s.sessions[hash] = session
				}
			}
		}
	}
	s.audits = append(s.audits, event)
	return nil
}

func (s *MemoryStore) RevokeAdminSessions(_ context.Context, adminID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for hash, session := range s.sessions {
		if session.AdminID == adminID && session.RevokedAt == nil {
			session.RevokedAt = &now
			s.sessions[hash] = session
		}
	}
	return nil
}
func (s *MemoryStore) FindAdminByEmail(_ context.Context, email string) (Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.admins {
		if strings.EqualFold(a.Email, email) {
			return a, nil
		}
	}
	return Admin{}, ErrNotFound
}
func (s *MemoryStore) FindAdminByID(_ context.Context, id string) (Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.admins[id]
	if !ok {
		return Admin{}, ErrNotFound
	}
	return a, nil
}
func (s *MemoryStore) RecordLoginFailure(context.Context, string) error { return nil }
func (s *MemoryStore) RecordLoginSuccess(context.Context, string) error { return nil }
func (s *MemoryStore) CreateSession(_ context.Context, session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.TokenHash] = session
	return nil
}
func (s *MemoryStore) RotateSession(_ context.Context, oldHash, newHash, newID string, expires time.Time) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.sessions[oldHash]
	if !ok {
		return Session{}, ErrNotFound
	}
	if old.RevokedAt != nil || time.Now().After(old.ExpiresAt) {
		for hash, session := range s.sessions {
			if session.FamilyID == old.FamilyID {
				now := time.Now()
				session.RevokedAt = &now
				s.sessions[hash] = session
			}
		}
		return Session{}, ErrNotFound
	}
	now := time.Now()
	old.RevokedAt = &now
	s.sessions[oldHash] = old
	next := Session{ID: newID, FamilyID: old.FamilyID, AdminID: old.AdminID, TokenHash: newHash, ExpiresAt: expires, CreatedAt: old.CreatedAt, IPAddress: old.IPAddress, UserAgent: old.UserAgent, MFAVerified: old.MFAVerified}
	s.sessions[newHash] = next
	return next, nil
}
func (s *MemoryStore) RevokeSession(_ context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[hash]
	if !ok {
		return nil
	}
	now := time.Now()
	session.RevokedAt = &now
	s.sessions[hash] = session
	return nil
}
func (s *MemoryStore) ListSessions(_ context.Context, adminID string) ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var sessions []Session
	for _, session := range s.sessions {
		if session.AdminID == adminID && session.RevokedAt == nil && time.Now().Before(session.ExpiresAt) {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}
func (s *MemoryStore) RevokeSessionByID(_ context.Context, adminID, id string, event AuditEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, session := range s.sessions {
		if session.AdminID == adminID && session.ID == id && session.RevokedAt == nil {
			now := time.Now()
			session.RevokedAt = &now
			s.sessions[hash] = session
			s.audits = append(s.audits, event)
			return true, nil
		}
	}
	return false, nil
}
func (s *MemoryStore) RevokeOtherSessions(_ context.Context, adminID, currentID string, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for hash, session := range s.sessions {
		if session.AdminID == adminID && session.ID != currentID && session.RevokedAt == nil {
			session.RevokedAt = &now
			s.sessions[hash] = session
		}
	}
	s.audits = append(s.audits, event)
	return nil
}
func (s *MemoryStore) SessionActive(_ context.Context, id, adminID string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session.ID == id && session.AdminID == adminID && session.RevokedAt == nil && time.Now().Before(session.ExpiresAt) {
			return session, nil
		}
	}
	return Session{}, ErrNotFound
}
func (s *MemoryStore) SavePendingMFA(_ context.Context, adminID string, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mfa[adminID] = append([]byte(nil), secret...)
	return nil
}
func (s *MemoryStore) MFASecret(_ context.Context, adminID string, verified bool) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	secret, ok := s.mfa[adminID]
	if !ok || (verified && !s.verified[adminID]) {
		return nil, ErrNotFound
	}
	return append([]byte(nil), secret...), nil
}
func (s *MemoryStore) ConfirmMFA(_ context.Context, adminID string, hashes []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verified[adminID] = true
	s.recovery[adminID] = append([]string(nil), hashes...)
	a := s.admins[adminID]
	a.MFAEnrolled = true
	s.admins[adminID] = a
	return nil
}
func (s *MemoryStore) UseRecoveryCode(_ context.Context, adminID, code string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, hash := range s.recovery[adminID] {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			s.recovery[adminID] = append(s.recovery[adminID][:i], s.recovery[adminID][i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (s *MemoryStore) AppendAudit(_ context.Context, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits = append(s.audits, event)
	return nil
}
func (s *MemoryStore) ListAuditEvents(_ context.Context, filter AuditFilter) (AuditEventPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	events := make([]AuditEvent, 0, limit)
	offset := filter.Offset
	for i := len(s.audits) - 1; i >= 0; i-- {
		event := s.audits[i]
		if filter.ActorID != "" && event.AdminID != filter.ActorID || filter.Action != "" && event.Action != filter.Action || filter.ResourceType != "" && event.ResourceType != filter.ResourceType || filter.ResourceID != "" && event.ResourceID != filter.ResourceID || filter.Outcome != "" && event.Outcome != filter.Outcome || filter.From != nil && event.OccurredAt.Before(*filter.From) || filter.To != nil && !event.OccurredAt.Before(*filter.To) {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		events = append(events, event)
		if len(events) == limit {
			break
		}
	}
	return AuditEventPage{Events: events, Limit: limit, Offset: filter.Offset}, nil
}
func (s *MemoryStore) SearchUsers(_ context.Context, query string, limit, offset int, sensitive bool) (UserPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query = strings.ToLower(query)
	users := make([]User, 0, limit)
	for _, detail := range s.users {
		user := detail.User
		if query != "" && !strings.Contains(strings.ToLower(user.ID), query) && !strings.Contains(strings.ToLower(user.Username), query) && !strings.Contains(strings.ToLower(user.DisplayName), query) && (!sensitive || !strings.Contains(strings.ToLower(user.Email), query)) {
			continue
		}
		if !sensitive {
			user.Email = ""
		}
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.After(users[j].CreatedAt) || users[i].CreatedAt.Equal(users[j].CreatedAt) && users[i].ID > users[j].ID
	})
	if offset > len(users) {
		offset = len(users)
	}
	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	return UserPage{Users: users[offset:end], Limit: limit, Offset: offset}, nil
}

func (s *MemoryStore) SearchRooms(_ context.Context, filter RoomFilter) (RoomPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rooms := make([]Room, 0, filter.Limit)
	for _, detail := range s.rooms {
		room := detail.Room
		if filter.ID != "" && room.ID != filter.ID || filter.InviteCode != "" && room.InviteCode != filter.InviteCode || filter.Status != "" && room.Status != filter.Status || filter.Visibility != "" && room.Visibility != filter.Visibility || filter.Mode != "" && room.GameMode != filter.Mode || filter.CreatedFrom != nil && room.CreatedAt.Before(*filter.CreatedFrom) || filter.CreatedTo != nil && !room.CreatedAt.Before(*filter.CreatedTo) {
			continue
		}
		room.PlayerCount = len(detail.Players)
		rooms = append(rooms, room)
	}
	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i].CreatedAt.After(rooms[j].CreatedAt) || rooms[i].CreatedAt.Equal(rooms[j].CreatedAt) && rooms[i].ID > rooms[j].ID
	})
	if filter.Offset > len(rooms) {
		filter.Offset = len(rooms)
	}
	end := filter.Offset + filter.Limit
	if end > len(rooms) {
		end = len(rooms)
	}
	return RoomPage{Rooms: rooms[filter.Offset:end], Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *MemoryStore) GetRoom(_ context.Context, id string) (RoomDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	detail, ok := s.rooms[id]
	if !ok {
		return RoomDetail{}, ErrNotFound
	}
	detail.Room.PlayerCount = len(detail.Players)
	return detail, nil
}

func (s *MemoryStore) SearchGames(_ context.Context, filter GameFilter) (GamePage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	games := make([]Game, 0, filter.Limit)
	for _, detail := range s.games {
		game := detail.Game
		if filter.ID != "" && game.ID != filter.ID || filter.RoomID != "" && game.RoomID != filter.RoomID || filter.Mode != "" && game.Mode != filter.Mode || filter.SeasonID != "" && game.SeasonID != filter.SeasonID || filter.Completion == "completed" && game.FinishedAt == nil || filter.FinishedFrom != nil && (game.FinishedAt == nil || game.FinishedAt.Before(*filter.FinishedFrom)) || filter.FinishedTo != nil && (game.FinishedAt == nil || !game.FinishedAt.Before(*filter.FinishedTo)) {
			continue
		}
		if filter.PlayerID != "" {
			found := false
			for _, player := range detail.Players {
				if player.UserID == filter.PlayerID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		games = append(games, game)
	}
	sort.Slice(games, func(i, j int) bool {
		return games[i].FinishedAt != nil && (games[j].FinishedAt == nil || games[i].FinishedAt.After(*games[j].FinishedAt)) || games[i].FinishedAt == nil && games[j].FinishedAt != nil || games[i].FinishedAt != nil && games[j].FinishedAt != nil && games[i].FinishedAt.Equal(*games[j].FinishedAt) && games[i].ID > games[j].ID
	})
	total := len(games)
	if filter.Offset > len(games) {
		filter.Offset = len(games)
	}
	end := filter.Offset + filter.Limit
	if end > len(games) {
		end = len(games)
	}
	return GamePage{Games: games[filter.Offset:end], Limit: filter.Limit, Offset: filter.Offset, Total: total}, nil
}
func (s *MemoryStore) GetGame(_ context.Context, id string) (GameDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[id]
	if !ok {
		return GameDetail{}, ErrNotFound
	}
	if game.Flags == nil {
		game.Flags = []GameFlag{}
	}
	if game.Notes == nil {
		game.Notes = []GameNote{}
	}
	return game, nil
}
func (s *MemoryStore) FlagGame(_ context.Context, id, reason string, event AuditEvent) (GameFlag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	detail, ok := s.games[id]
	if !ok {
		return GameFlag{}, ErrNotFound
	}
	flag := GameFlag{ID: uuid.NewString(), Reason: reason, CreatedBy: event.AdminID, CreatedAt: time.Now()}
	detail.Flags = append(detail.Flags, flag)
	s.games[id] = detail
	s.audits = append(s.audits, event)
	return flag, nil
}
func (s *MemoryStore) AddGameNote(_ context.Context, id, reason, body string, event AuditEvent) (GameNote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	detail, ok := s.games[id]
	if !ok {
		return GameNote{}, ErrNotFound
	}
	note := GameNote{ID: uuid.NewString(), Reason: reason, Body: body, CreatedBy: event.AdminID, CreatedAt: time.Now()}
	detail.Notes = append(detail.Notes, note)
	s.games[id] = detail
	s.audits = append(s.audits, event)
	return note, nil
}
func (s *MemoryStore) SetGames(games ...GameDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, game := range games {
		s.games[game.Game.ID] = game
	}
}

func (s *MemoryStore) SetRooms(rooms ...RoomDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, room := range rooms {
		s.rooms[room.Room.ID] = room
	}
}

func (s *MemoryStore) GetUser(_ context.Context, id string, sensitive bool) (UserDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	detail, ok := s.users[id]
	if !ok {
		return UserDetail{}, ErrNotFound
	}
	if !sensitive {
		detail.User.Email = ""
	}
	return detail, nil
}

func (s *MemoryStore) SuspendUser(_ context.Context, id string, suspension Suspension, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	user.User.Suspension = &suspension
	s.users[id] = user
	s.audits = append(s.audits, event)
	return nil
}

func (s *MemoryStore) ReinstateUser(_ context.Context, id string, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	user.User.Suspension = nil
	s.users[id] = user
	s.audits = append(s.audits, event)
	return nil
}

func (s *MemoryStore) UpdateUserDisplayName(_ context.Context, id, displayName string, version int, event AuditEvent) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	detail, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	if detail.User.Version != version {
		return User{}, ErrConflict
	}
	beforeName, beforeVersion := detail.User.DisplayName, detail.User.Version
	detail.User.DisplayName = displayName
	detail.User.Version++
	event.BeforeState = []byte(fmt.Sprintf(`{"display_name":%q,"version":%d}`, beforeName, beforeVersion))
	event.AfterState = []byte(fmt.Sprintf(`{"display_name":%q,"version":%d}`, displayName, detail.User.Version))
	s.users[id] = detail
	s.audits = append(s.audits, event)
	return detail.User, nil
}

func (s *MemoryStore) SetUsers(users ...UserDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range users {
		s.users[user.User.ID] = user
	}
}

func (s *MemoryStore) Dashboard(context.Context) (Dashboard, error) {
	now := time.Now().UTC()
	day := model.TimeWindow{From: now.Truncate(24 * time.Hour), To: now.Truncate(24 * time.Hour).Add(24 * time.Hour)}
	month := model.TimeWindow{From: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), To: time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)}
	return Dashboard{
		Status: "ready", Environment: "development", Windows: model.DashboardWindows{Day: day, Month: month},
		Services: model.DashboardServices{API: model.ServiceHealth{Status: "ok"}, WS: model.ServiceHealth{Status: "not_configured"}},
	}, nil
}
func (s *MemoryStore) SetPermissions(id string, permissions []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.admins[id]
	a.Permissions = permissions
	s.admins[id] = a
}
func (s *MemoryStore) AuditEvents() []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AuditEvent(nil), s.audits...)
}
