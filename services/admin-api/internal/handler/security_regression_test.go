package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMemoryMFAProtectsVerifiedState(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(Admin{ID: "admin"})
	if err := store.ConfirmMFA(ctx, "admin", nil, AuditEvent{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("confirmation without pending secret = %v", err)
	}
	for _, secret := range []string{"first", "replacement"} {
		if err := store.SavePendingMFA(ctx, "admin", []byte(secret), AuditEvent{Action: "admin.mfa.enroll"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ConfirmMFA(ctx, "admin", []string{"original-recovery"}, AuditEvent{Action: "admin.mfa.confirm"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePendingMFA(ctx, "admin", []byte("attacker"), AuditEvent{}); !errors.Is(err, ErrConflict) {
		t.Errorf("verified replacement = %v", err)
	}
	if err := store.ConfirmMFA(ctx, "admin", []string{"new-recovery"}, AuditEvent{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("reconfirmation = %v", err)
	}
	secret, err := store.MFASecret(ctx, "admin", true)
	if err != nil || string(secret) != "replacement" || store.recovery["admin"][0] != "original-recovery" {
		t.Fatalf("verified state changed: secret=%q err=%v", secret, err)
	}
}

func TestMemoryMFACredentialChangesRollBackWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(Admin{ID: "admin"})
	invalidAudit := AuditEvent{Metadata: make([]byte, 16*1024+1)}

	if err := store.SavePendingMFA(ctx, "admin", []byte("secret"), invalidAudit); err == nil {
		t.Fatal("SavePendingMFA succeeded without its audit event")
	}
	if _, err := store.MFASecret(ctx, "admin", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending credential was not rolled back: %v", err)
	}
	if err := store.SavePendingMFA(ctx, "admin", []byte("secret"), AuditEvent{Action: "admin.mfa.enroll"}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmMFA(ctx, "admin", []string{"recovery-hash"}, invalidAudit); err == nil {
		t.Fatal("ConfirmMFA succeeded without its audit event")
	}
	if store.verified["admin"] || len(store.recovery["admin"]) != 0 || store.admins["admin"].MFAEnrolled {
		t.Fatal("confirmed credential state was not rolled back")
	}
}

type pendingMFAErrorStore struct {
	Store
	err error
}

func (s pendingMFAErrorStore) SavePendingMFA(context.Context, string, []byte, AuditEvent) error {
	return s.err
}

func TestEnrollMFAHandlesPersistenceConflict(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"verified concurrently", fmt.Errorf("save: %w", ErrConflict), http.StatusConflict},
		{"dependency failure", errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAdminHandler(Config{MFAEncryptionKey: "test-mfa-key-at-least-32-bytes!!"}, pendingMFAErrorStore{Store: NewMemoryStore(), err: tc.err}, Dependencies{})
			r := gin.New()
			r.POST("/enroll", func(c *gin.Context) { c.Set("admin", Admin{ID: "admin", Email: "ops@example.com"}); h.EnrollMFA(c) })
			response := request(t, r, http.MethodPost, "/enroll", `{}`, "")
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

type invitationLookupStore struct {
	Store
	err     error
	lookups int
	accepts int
}

func (s *invitationLookupStore) FindInvitationByTokenHash(context.Context, string) (Invitation, error) {
	s.lookups++
	return Invitation{}, s.err
}

func (s *invitationLookupStore) AcceptInvitation(context.Context, string, string, string, AuditEvent) (Admin, error) {
	s.accepts++
	return Admin{}, nil
}

func TestInvitationLookupErrorsAndRateLimit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"invalid token", fmt.Errorf("lookup: %w", ErrNotFound), http.StatusNotFound},
		{"dependency failure", errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &invitationLookupStore{Store: NewMemoryStore(), err: tc.err}
			r := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
			for i := 0; i < 6; i++ {
				response := request(t, r, http.MethodPost, "/auth/invitations/accept", `{"token":"invalid","display_name":"Operator","password":"correct horse battery staple"}`, "")
				want := tc.status
				if i == 5 {
					want = http.StatusTooManyRequests
				}
				if response.Code != want {
					t.Fatalf("attempt %d: status=%d body=%s", i, response.Code, response.Body.String())
				}
			}
			if store.lookups != 5 || store.accepts != 0 {
				t.Fatalf("lookups=%d accepts=%d", store.lookups, store.accepts)
			}
		})
	}
}
