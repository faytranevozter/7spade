package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const refreshCookieName = "admin_refresh_token"

var ErrNotFound = errors.New("not found")

type Config struct {
	JWTSecret     string
	SecureCookies bool
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	AllowedOrigin string
}

type Admin struct {
	ID           string   `json:"id"`
	Email        string   `json:"email"`
	DisplayName  string   `json:"display_name"`
	PasswordHash string   `json:"-"`
	Status       string   `json:"status"`
	Permissions  []string `json:"permissions"`
}

type Session struct {
	ID        string
	FamilyID  string
	AdminID   string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type AuditEvent struct {
	AdminID    string
	SessionID  string
	RequestID  string
	Action     string
	Outcome    string
	IPAddress  string
	UserAgent  string
	OccurredAt time.Time
}

type Dashboard struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
}

type Store interface {
	FindAdminByEmail(context.Context, string) (Admin, error)
	FindAdminByID(context.Context, string) (Admin, error)
	RecordLoginFailure(context.Context, string) error
	RecordLoginSuccess(context.Context, string) error
	CreateSession(context.Context, Session) error
	RotateSession(context.Context, string, string, string, time.Time) (Session, error)
	RevokeSession(context.Context, string) error
	SessionActive(context.Context, string, string) (bool, error)
	AppendAudit(context.Context, AuditEvent) error
	Dashboard(context.Context) (Dashboard, error)
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	Admin       Admin  `json:"admin"`
}

type accessClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

type handler struct {
	cfg      Config
	store    Store
	attempts *loginAttempts
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

func NewRouter(cfg Config, store Store) *gin.Engine {
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery(), func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	})
	if cfg.AllowedOrigin != "" {
		r.Use(cors.New(cors.Config{AllowOrigins: []string{cfg.AllowedOrigin}, AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Authorization", "Content-Type"}, AllowCredentials: true}))
	}
	h := handler{cfg: cfg, store: store, attempts: &loginAttempts{entries: map[string][]time.Time{}}}
	r.POST("/auth/login", h.login)
	r.POST("/auth/refresh", h.refresh)
	r.DELETE("/auth/logout", h.logout)
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "admin-api"}) })
	authed := r.Group("")
	authed.Use(h.requireAuth)
	authed.GET("/me", h.me)
	authed.GET("/dashboard", h.requirePermission("dashboard.read"), h.dashboard)
	return r
}

func (h handler) login(c *gin.Context) {
	if !h.attempts.allow(c.ClientIP(), time.Now()) {
		jsonError(c, http.StatusTooManyRequests, "Too many login attempts")
		return
	}
	var req struct{ Email, Password string }
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
	h.issueSession(c, admin, "admin.login")
}

func (h handler) refresh(c *gin.Context) {
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
		h.clearCookie(c)
		jsonError(c, http.StatusUnauthorized, "Invalid session")
		return
	}
	access, err := h.accessToken(admin.ID, session.ID)
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

func (h handler) logout(c *gin.Context) {
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

func (h handler) issueSession(c *gin.Context, admin Admin, action string) {
	raw, err := randomToken()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	session := Session{ID: uuid.NewString(), FamilyID: uuid.NewString(), AdminID: admin.ID, TokenHash: hashToken(raw), ExpiresAt: time.Now().Add(h.cfg.RefreshTTL)}
	if err := h.store.CreateSession(c, session); err != nil {
		jsonError(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	access, err := h.accessToken(admin.ID, session.ID)
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

func (h handler) accessToken(adminID, sessionID string) (string, error) {
	now := time.Now()
	claims := accessClaims{SessionID: sessionID, RegisteredClaims: jwt.RegisteredClaims{Subject: adminID, Issuer: "seven-spade-admin", Audience: jwt.ClaimStrings{"admin-api"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.AccessTTL))}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.JWTSecret))
}

func (h handler) requireAuth(c *gin.Context) {
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
	active, sessionErr := h.store.SessionActive(c, claims.SessionID, claims.Subject)
	if err != nil || sessionErr != nil || !active || admin.Status != "active" {
		jsonError(c, http.StatusUnauthorized, "Authentication required")
		c.Abort()
		return
	}
	c.Set("admin", admin)
	c.Set("session_id", claims.SessionID)
	c.Next()
}

func (h handler) requirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		admin := c.MustGet("admin").(Admin)
		for _, value := range admin.Permissions {
			if value == permission {
				c.Next()
				return
			}
		}
		jsonError(c, http.StatusForbidden, "Permission denied")
		c.Abort()
	}
}

func (h handler) me(c *gin.Context) { c.JSON(http.StatusOK, c.MustGet("admin")) }
func (h handler) dashboard(c *gin.Context) {
	result, err := h.store.Dashboard(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load dashboard")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handler) setCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, token, int(h.cfg.RefreshTTL.Seconds()), "/auth", "", h.cfg.SecureCookies, true)
}
func (h handler) clearCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, "/auth", "", h.cfg.SecureCookies, true)
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
	mu       sync.Mutex
	admins   map[string]Admin
	sessions map[string]Session
	audits   []AuditEvent
}

func NewMemoryStore(admins ...Admin) *MemoryStore {
	s := &MemoryStore{admins: map[string]Admin{}, sessions: map[string]Session{}}
	for _, a := range admins {
		s.admins[a.ID] = a
	}
	return s
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
	next := Session{ID: newID, FamilyID: old.FamilyID, AdminID: old.AdminID, TokenHash: newHash, ExpiresAt: expires}
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
func (s *MemoryStore) SessionActive(_ context.Context, id, adminID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session.ID == id && session.AdminID == adminID {
			return session.RevokedAt == nil && time.Now().Before(session.ExpiresAt), nil
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
func (s *MemoryStore) Dashboard(context.Context) (Dashboard, error) {
	return Dashboard{Status: "ready", Environment: "development"}, nil
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
