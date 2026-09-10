package room

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIClientsSendInternalSecretHeader(t *testing.T) {
	gotHeader := make(chan string, 1)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("X-Internal-Secret")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	server := NewGameServerWithOptions(
		Config{JWTSecret: "test-secret", APIURL: apiServer.URL, InternalSecret: "top-secret"},
		newMemoryStateStore(),
		time.Hour,
	)

	if err := server.reconciler.ReconcileRooms([]string{"room-x"}); err != nil {
		t.Fatalf("reconcile rooms: %v", err)
	}
	select {
	case h := <-gotHeader:
		if h != "top-secret" {
			t.Fatalf("X-Internal-Secret = %q, want %q", h, "top-secret")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reconcile request")
	}
}

func TestAPIClientsOmitInternalSecretWhenUnset(t *testing.T) {
	gotHeader := make(chan string, 1)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("X-Internal-Secret")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	server := NewGameServerWithOptions(
		Config{JWTSecret: "test-secret", APIURL: apiServer.URL},
		newMemoryStateStore(),
		time.Hour,
	)

	if err := server.reconciler.ReconcileRooms([]string{"room-x"}); err != nil {
		t.Fatalf("reconcile rooms: %v", err)
	}
	select {
	case h := <-gotHeader:
		if h != "" {
			t.Fatalf("expected no X-Internal-Secret header, got %q", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reconcile request")
	}
}
