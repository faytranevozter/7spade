package model

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrUserActive = errors.New("user active")
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
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
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
	ID           string
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

type TimeWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type DashboardWindows struct {
	Day   TimeWindow `json:"day"`
	Month TimeWindow `json:"month"`
}

type ActivitySummary struct {
	Registrations              int64 `json:"registrations"`
	Players                    int64 `json:"players"`
	Rooms                      int64 `json:"rooms"`
	GamesStarted               int64 `json:"games_started"`
	GamesCompleted             int64 `json:"games_completed"`
	GamesAbandoned             int64 `json:"games_abandoned"`
	AverageGameDurationSeconds int64 `json:"average_game_duration_seconds"`
}

type CurrentActivity struct {
	Players int64 `json:"players"`
	Rooms   int64 `json:"rooms"`
	Games   int64 `json:"games"`
}

type ServiceHealth struct {
	Status string `json:"status"`
}

type DashboardServices struct {
	API ServiceHealth `json:"api"`
	WS  ServiceHealth `json:"ws"`
}

type OperationsLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Dashboard struct {
	Status      string            `json:"status"`
	Environment string            `json:"environment"`
	Windows     DashboardWindows  `json:"windows"`
	Current     CurrentActivity   `json:"current"`
	Daily       ActivitySummary   `json:"daily"`
	Monthly     ActivitySummary   `json:"monthly"`
	Services    DashboardServices `json:"services"`
	Links       []OperationsLink  `json:"links"`
}

type User struct {
	ID          string      `json:"id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	Version     int         `json:"version"`
	Email       string      `json:"email,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	Online      bool        `json:"online"`
	Suspension  *Suspension `json:"suspension,omitempty"`
}

type Suspension struct {
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type UserPage struct {
	Users  []User `json:"users"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type UserDetail struct {
	User         User             `json:"user"`
	Providers    []string         `json:"providers"`
	Stats        map[string]any   `json:"stats"`
	Ratings      []map[string]any `json:"ratings"`
	Achievements []map[string]any `json:"achievements"`
	Skins        []map[string]any `json:"skins"`
	Games        []map[string]any `json:"games"`
	Room         map[string]any   `json:"room,omitempty"`
}
