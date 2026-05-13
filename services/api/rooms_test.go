package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// setupRoomsTestDB extends setupTestDB by also clearing room-related tables.
func setupRoomsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := setupTestDB(t)
	_, _ = db.Exec("DELETE FROM room_players")
	_, _ = db.Exec("DELETE FROM rooms")
	return db
}

// createTestUser inserts a user and returns the resulting ID.
func createTestUser(t *testing.T, db *sql.DB, email, displayName string) uuid.UUID {
	t.Helper()
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := CreateUser(db, email, hash, displayName)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user.ID
}

// authedRequest builds a request with the given user's claims attached via context.
func authedRequest(method, target string, body []byte, userID uuid.UUID, displayName string) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, bytes.NewBuffer(body))
	}
	claims := &Claims{Sub: userID.String(), DisplayName: displayName, IsGuest: false}
	ctx := context.WithValue(req.Context(), claimsContextKey, claims)
	return req.WithContext(ctx)
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{"valid bearer", "Bearer abc.def.ghi", "abc.def.ghi", true},
		{"missing prefix", "abc.def.ghi", "", false},
		{"empty header", "", "", false},
		{"prefix only", "Bearer ", "", false},
		{"prefix with whitespace token", "Bearer    ", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, ok := extractBearerToken(tt.header)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tok != tt.wantToken {
				t.Errorf("token = %q, want %q", tok, tt.wantToken)
			}
		})
	}
}

func TestRequireAuthRejectsMissingToken(t *testing.T) {
	handler := requireAuth("test-secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthRejectsInvalidToken(t *testing.T) {
	handler := requireAuth("test-secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthAcceptsValidToken(t *testing.T) {
	secret := "test-secret"
	token, err := GenerateGuestToken("Tester", secret)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	called := false
	handler := requireAuth(secret, func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			t.Error("expected claims in context")
		}
		if claims.DisplayName != "Tester" {
			t.Errorf("display name = %q, want %q", claims.DisplayName, "Tester")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCreateRoomHandlerSuccess(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	userID := createTestUser(t, db, "creator@example.com", "Creator")

	body := []byte(`{"visibility":"public","turn_timer_seconds":60}`)
	req := authedRequest(http.MethodPost, "/rooms", body, userID, "Creator")
	rec := httptest.NewRecorder()
	createRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp roomResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID == "" {
		t.Error("expected non-empty room ID")
	}
	if len(resp.InviteCode) != 6 {
		t.Errorf("expected 6-char invite code, got %q", resp.InviteCode)
	}
	if resp.Visibility != "public" {
		t.Errorf("visibility = %q, want public", resp.Visibility)
	}
	if resp.TurnTimerSeconds != 60 {
		t.Errorf("turn_timer = %d, want 60", resp.TurnTimerSeconds)
	}
	if resp.Status != "waiting" {
		t.Errorf("status = %q, want waiting", resp.Status)
	}
	if resp.PlayerCount != 1 {
		t.Errorf("player_count = %d, want 1", resp.PlayerCount)
	}
}

func TestCreateRoomHandlerRejectsInvalidVisibility(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	userID := createTestUser(t, db, "creator@example.com", "Creator")
	body := []byte(`{"visibility":"unknown","turn_timer_seconds":60}`)
	req := authedRequest(http.MethodPost, "/rooms", body, userID, "Creator")
	rec := httptest.NewRecorder()
	createRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateRoomHandlerRejectsInvalidTimer(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	userID := createTestUser(t, db, "creator@example.com", "Creator")
	body := []byte(`{"visibility":"public","turn_timer_seconds":45}`)
	req := authedRequest(http.MethodPost, "/rooms", body, userID, "Creator")
	rec := httptest.NewRecorder()
	createRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListPublicRoomsHandlerOnlyReturnsPublicWaiting(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	creatorID := createTestUser(t, db, "creator@example.com", "Creator")

	// Public waiting room (should appear)
	pub, err := CreateRoom(db, "public", 60, creatorID)
	if err != nil {
		t.Fatalf("CreateRoom public: %v", err)
	}
	// Private waiting room (should NOT appear)
	if _, err := CreateRoom(db, "private", 60, creatorID); err != nil {
		t.Fatalf("CreateRoom private: %v", err)
	}
	// Public in_progress room (should NOT appear)
	inProg, err := CreateRoom(db, "public", 60, creatorID)
	if err != nil {
		t.Fatalf("CreateRoom in-prog: %v", err)
	}
	if _, err := db.Exec("UPDATE rooms SET status = 'in_progress' WHERE id = $1", inProg.ID); err != nil {
		t.Fatalf("update room: %v", err)
	}

	rec := httptest.NewRecorder()
	listPublicRoomsHandler(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/rooms", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var rooms []roomResponse
	if err := json.NewDecoder(rec.Body).Decode(&rooms); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms))
	}
	if rooms[0].ID != pub.ID.String() {
		t.Errorf("returned wrong room: got %s, want %s", rooms[0].ID, pub.ID.String())
	}
}

func TestJoinRoomHandlerSuccess(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	creatorID := createTestUser(t, db, "creator@example.com", "Creator")
	joinerID := createTestUser(t, db, "joiner@example.com", "Joiner")

	room, err := CreateRoom(db, "public", 60, creatorID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	req := authedRequest(http.MethodPost, "/rooms/"+room.InviteCode+"/join", nil, joinerID, "Joiner")
	req.SetPathValue("code", room.InviteCode)
	rec := httptest.NewRecorder()
	joinRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp joinRoomResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != room.ID.String() {
		t.Errorf("room ID = %s, want %s", resp.ID, room.ID.String())
	}
	if resp.PlayerCount != 1 {
		t.Errorf("player_count = %d, want 1", resp.PlayerCount)
	}
	if resp.Status != "waiting" {
		t.Errorf("status = %q, want waiting", resp.Status)
	}
}

func TestJoinRoomHandlerRejectsFullRoom(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	creatorID := createTestUser(t, db, "creator@example.com", "Creator")
	room, err := CreateRoom(db, "public", 60, creatorID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	// Fill the room with 4 players (transitions to in_progress on 4th)
	emails := []string{"p1@example.com", "p2@example.com", "p3@example.com", "p4@example.com"}
	names := []string{"P1", "P2", "P3", "P4"}
	for i, email := range emails {
		uid := createTestUser(t, db, email, names[i])
		if _, err := AddPlayerToRoom(db, room.ID, uid, names[i]); err != nil {
			t.Fatalf("AddPlayerToRoom #%d: %v", i+1, err)
		}
	}

	// 5th player tries to join — should be rejected.
	fifthID := createTestUser(t, db, "p5@example.com", "P5")
	req := authedRequest(http.MethodPost, "/rooms/"+room.InviteCode+"/join", nil, fifthID, "P5")
	req.SetPathValue("code", room.InviteCode)
	rec := httptest.NewRecorder()
	joinRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestJoinRoomHandlerTransitionsToInProgressOn4thJoin(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	creatorID := createTestUser(t, db, "creator@example.com", "Creator")
	room, err := CreateRoom(db, "public", 60, creatorID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	// Add creator + 2 more pre-existing players (3 total).
	if _, err := AddPlayerToRoom(db, room.ID, creatorID, "Creator"); err != nil {
		t.Fatalf("creator join: %v", err)
	}
	for i, email := range []string{"p2@example.com", "p3@example.com"} {
		uid := createTestUser(t, db, email, "P"+string(rune('2'+i)))
		if _, err := AddPlayerToRoom(db, room.ID, uid, "P"+string(rune('2'+i))); err != nil {
			t.Fatalf("AddPlayerToRoom: %v", err)
		}
	}

	// 4th player joins via the handler — room should transition to in_progress.
	fourthID := createTestUser(t, db, "p4@example.com", "P4")
	req := authedRequest(http.MethodPost, "/rooms/"+room.InviteCode+"/join", nil, fourthID, "P4")
	req.SetPathValue("code", room.InviteCode)
	rec := httptest.NewRecorder()
	joinRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp joinRoomResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PlayerCount != 4 {
		t.Errorf("player_count = %d, want 4", resp.PlayerCount)
	}
	if resp.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", resp.Status)
	}
}

func TestJoinRoomHandlerReturns404ForUnknownCode(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	userID := createTestUser(t, db, "user@example.com", "User")
	req := authedRequest(http.MethodPost, "/rooms/NOPE99/join", nil, userID, "User")
	req.SetPathValue("code", "NOPE99")
	rec := httptest.NewRecorder()
	joinRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetRoomHandlerReturnsRoomDetails(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	creatorID := createTestUser(t, db, "creator@example.com", "Creator")
	room, err := CreateRoom(db, "public", 90, creatorID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, err := AddPlayerToRoom(db, room.ID, creatorID, "Creator"); err != nil {
		t.Fatalf("AddPlayer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/rooms/"+room.ID.String(), nil)
	req.SetPathValue("id", room.ID.String())
	rec := httptest.NewRecorder()
	getRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp roomResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != room.ID.String() {
		t.Errorf("ID mismatch")
	}
	if resp.PlayerCount != 1 {
		t.Errorf("player_count = %d, want 1", resp.PlayerCount)
	}
	if resp.TurnTimerSeconds != 90 {
		t.Errorf("turn_timer = %d, want 90", resp.TurnTimerSeconds)
	}
}

func TestGetRoomHandlerReturns404ForUnknownID(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/rooms/"+uuid.New().String(), nil)
	req.SetPathValue("id", uuid.New().String())
	rec := httptest.NewRecorder()
	getRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetRoomHandlerReturns400ForInvalidID(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/rooms/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()
	getRoomHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGenerateInviteCodeProducesUniqueCodes(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := GenerateInviteCode()
		if err != nil {
			t.Fatalf("GenerateInviteCode: %v", err)
		}
		if len(code) != 6 {
			t.Errorf("expected length 6, got %d (%q)", len(code), code)
		}
		if seen[code] {
			t.Errorf("duplicate code generated: %q", code)
		}
		seen[code] = true
	}
}
