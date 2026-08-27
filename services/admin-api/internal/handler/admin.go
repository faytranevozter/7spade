package handler

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
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
	Admin      = model.Admin
	Session    = model.Session
	AuditEvent = model.AuditEvent
	Dashboard  = model.Dashboard
)

var ErrNotFound = model.ErrNotFound

type Store interface {
	FindAdminByEmail(context.Context, string) (Admin, error)
	FindAdminByID(context.Context, string) (Admin, error)
	RecordLoginFailure(context.Context, string) error
	RecordLoginSuccess(context.Context, string) error
	CreateSession(context.Context, Session) error
	RotateSession(context.Context, string, string, string, time.Time) (Session, error)
	RevokeSession(context.Context, string) error
	SessionActive(context.Context, string, string) (Session, error)
	SavePendingMFA(context.Context, string, []byte) error
	MFASecret(context.Context, string, bool) ([]byte, error)
	ConfirmMFA(context.Context, string, []string) error
	UseRecoveryCode(context.Context, string, string) (bool, error)
	AppendAudit(context.Context, AuditEvent) error
	Dashboard(context.Context) (Dashboard, error)
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	Admin       Admin  `json:"admin"`
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

type MFAChallengeResponse struct {
	MFARequired    bool   `json:"mfa_required"`
	ChallengeToken string `json:"challenge_token"`
}

type AdminHandler struct {
	cfg         Config
	store       Store
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

func NewAdminHandler(cfg Config, store Store) *AdminHandler {
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	return &AdminHandler{
		cfg:         cfg,
		store:       store,
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
	session := Session{ID: uuid.NewString(), FamilyID: uuid.NewString(), AdminID: admin.ID, TokenHash: hashToken(raw), ExpiresAt: time.Now().Add(h.cfg.RefreshTTL), MFAVerified: mfaVerified}
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
		jsonError(c, http.StatusForbidden, "Permission denied")
		c.Abort()
	}
}

func (h *AdminHandler) Me(c *gin.Context) { c.JSON(http.StatusOK, c.MustGet("admin")) }
func (h *AdminHandler) Dashboard(c *gin.Context) {
	result, err := h.store.Dashboard(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load dashboard")
		return
	}
	c.JSON(http.StatusOK, result)
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
	mu       sync.Mutex
	admins   map[string]Admin
	sessions map[string]Session
	audits   []AuditEvent
	mfa      map[string][]byte
	verified map[string]bool
	recovery map[string][]string
}

func NewMemoryStore(admins ...Admin) *MemoryStore {
	s := &MemoryStore{admins: map[string]Admin{}, sessions: map[string]Session{}, mfa: map[string][]byte{}, verified: map[string]bool{}, recovery: map[string][]string{}}
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
	next := Session{ID: newID, FamilyID: old.FamilyID, AdminID: old.AdminID, TokenHash: newHash, ExpiresAt: expires, MFAVerified: old.MFAVerified}
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
