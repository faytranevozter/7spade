package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// All friend endpoints reject guests and missing claims before touching the DB,
// so these run with DB: nil.
func TestFriendsRejectNonRegisteredUsers(t *testing.T) {
	endpoints := []struct {
		name   string
		invoke func(h FriendsHandler, c *gin.Context)
	}{
		{"List", func(h FriendsHandler, c *gin.Context) { h.List(c) }},
		{"Accept", func(h FriendsHandler, c *gin.Context) { h.Accept(c) }},
		{"Remove", func(h FriendsHandler, c *gin.Context) { h.Remove(c) }},
		{"Block", func(h FriendsHandler, c *gin.Context) { h.Block(c) }},
	}
	cases := []struct {
		name    string
		claims  *auth.Claims
		setNil  bool
		wantMsg string
	}{
		{name: "no claims", setNil: true, wantMsg: "Authentication required"},
		{name: "guest", claims: &auth.Claims{Sub: uuid.NewString(), IsGuest: true}, wantMsg: "Logged-in user required"},
		{name: "bad sub", claims: &auth.Claims{Sub: "not-a-uuid"}, wantMsg: "Logged-in user required"},
	}

	for _, ep := range endpoints {
		for _, tc := range cases {
			t.Run(ep.name+"/"+tc.name, func(t *testing.T) {
				h := FriendsHandler{DB: nil}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodGet, "/friends", nil)
				c.Params = gin.Params{{Key: "userId", Value: uuid.NewString()}}
				if !tc.setNil {
					c.Set(middleware.ClaimsKey, tc.claims)
				}
				ep.invoke(h, c)
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401", w.Code)
				}
				assertErrorBody(t, w, tc.wantMsg)
			})
		}
	}
}

// Accept/Remove/Block validate the path param after auth, before the DB.
func TestFriendsRejectInvalidUserID(t *testing.T) {
	endpoints := map[string]func(h FriendsHandler, c *gin.Context){
		"Accept": func(h FriendsHandler, c *gin.Context) { h.Accept(c) },
		"Remove": func(h FriendsHandler, c *gin.Context) { h.Remove(c) },
		"Block":  func(h FriendsHandler, c *gin.Context) { h.Block(c) },
	}
	for name, invoke := range endpoints {
		t.Run(name, func(t *testing.T) {
			h := FriendsHandler{DB: nil}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/friends/x/accept", nil)
			c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString()})
			c.Params = gin.Params{{Key: "userId", Value: "not-a-uuid"}}
			invoke(h, c)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
			assertErrorBody(t, w, "Invalid user ID")
		})
	}
}

// SendRequest with neither user_id nor username is a 400.
func TestFriendsSendRequestRequiresTarget(t *testing.T) {
	h := FriendsHandler{DB: nil}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{}`
	c.Request = httptest.NewRequest(http.MethodPost, "/friends/requests", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString()})

	h.SendRequest(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var body2 struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body2)
	if body2.Error == "" {
		t.Fatal("expected an error message")
	}
}

// A syntactically invalid username is rejected as not-found before any DB
// access, so this runs with DB: nil.
func TestFriendsSendRequestInvalidUsername(t *testing.T) {
	h := FriendsHandler{DB: nil}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"username":"no spaces allowed"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/friends/requests", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString()})

	h.SendRequest(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	assertErrorBody(t, w, "No player with that username")
}
