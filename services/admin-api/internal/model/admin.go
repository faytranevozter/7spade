package model

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Admin struct {
	ID           string   `json:"id"`
	Email        string   `json:"email"`
	DisplayName  string   `json:"display_name"`
	PasswordHash string   `json:"-"`
	Status       string   `json:"status"`
	Permissions  []string `json:"permissions"`
	MFAEnrolled  bool     `json:"mfa_enrolled"`
}

type Session struct {
	ID          string
	FamilyID    string
	AdminID     string
	TokenHash   string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	MFAVerified bool
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
