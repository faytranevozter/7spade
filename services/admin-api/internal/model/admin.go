package model

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type Permission struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Admin struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	PasswordHash string    `json:"-"`
	Status       string    `json:"status"`
	Roles        []Role    `json:"roles,omitempty"`
	Permissions  []string  `json:"permissions"`
	MFAEnrolled  bool      `json:"mfa_enrolled"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

type Invitation struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	TokenHash  string     `json:"-"`
	RoleID     string     `json:"role_id"`
	RoleName   string     `json:"role_name,omitempty"`
	InvitedBy  string     `json:"invited_by,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Session struct {
	ID          string
	FamilyID    string
	AdminID     string
	TokenHash   string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
	IPAddress   string
	UserAgent   string
	MFAVerified bool
}

type AuditEvent struct {
	ID           string          `json:"id"`
	AdminID      string          `json:"actor_id,omitempty"`
	SessionID    string          `json:"session_id,omitempty"`
	RequestID    string          `json:"request_id,omitempty"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type,omitempty"`
	ResourceID   string          `json:"resource_id,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Outcome      string          `json:"outcome"`
	BeforeState  json.RawMessage `json:"before_state,omitempty"`
	AfterState   json.RawMessage `json:"after_state,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	IPAddress    string          `json:"ip_address,omitempty"`
	UserAgent    string          `json:"user_agent,omitempty"`
	OccurredAt   time.Time       `json:"occurred_at"`
}

type AuditFilter struct {
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      string
	From         *time.Time
	To           *time.Time
	Limit        int
	Offset       int
}

type AuditEventPage struct {
	Events []AuditEvent `json:"events"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

type Dashboard struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
}
