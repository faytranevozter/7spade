package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestGetSkinDetail(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"skins.read"}})
	skin := Skin{ID: "42395ffa-fc5f-4700-bdb7-713a501f7305", Name: "Starter", SkinType: "avatar_frame", AssetKey: "skins/frame.png", IsStarter: true, UnlockRulesLocked: true,
		UnlockRules: []model.SkinUnlockRule{{Name: "Win", RuleType: "game_condition", Conditions: json.RawMessage(`[{"metric":"is_winner","operator":"eq","value":"true"}]`)}},
		Revisions:   []SkinRevision{{ID: "revision-2", Version: 2, Enabled: true}, {ID: "revision-1", Version: 1, Enabled: false}},
	}
	store.SetSkins(skin, Skin{ID: "other", Name: "Unrelated"})
	for _, storage := range []StorageSigner{nil, &stubSkinSigner{}} {
		h := NewAdminHandler(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, Dependencies{Storage: storage})
		r := gin.New()
		r.POST("/auth/login", h.Login)
		g := r.Group("", h.RequireAuth)
		g.GET("/skins", h.RequirePermission("skins.read"), h.ListSkins)
		g.GET("/skins/:id", h.RequirePermission("skins.read"), h.GetSkin)
		login := request(t, r, http.MethodPost, "/auth/login", `{"email":"reader@example.com","password":"password"}`, "")
		var auth AuthResponse
		if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"skins/frame.png", ""} {
			skin.AssetKey = key
			store.SetSkins(skin)
			response := request(t, r, http.MethodGet, "/skins/"+skin.ID, "", auth.AccessToken)
			if response.Code != http.StatusOK {
				t.Fatalf("detail: %d %s", response.Code, response.Body.String())
			}
			var got Skin
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := skin
			if storage != nil && key != "" {
				want.AssetURL = storage.PublicURL(key)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("detail=%+v want=%+v", got, want)
			}
			var list struct {
				Skins []Skin `json:"skins"`
			}
			listed := request(t, r, http.MethodGet, "/skins", "", auth.AccessToken)
			if err := json.Unmarshal(listed.Body.Bytes(), &list); err != nil {
				t.Fatal(err)
			}
			for _, entry := range list.Skins {
				if entry.ID == skin.ID && !reflect.DeepEqual(entry, got) {
					t.Fatal("detail differs from list entry")
				}
			}
		}
		response := request(t, r, http.MethodGet, "/skins/missing", "", auth.AccessToken)
		if response.Code != http.StatusNotFound || response.Body.String() != `{"error":"Skin not found"}` {
			t.Fatalf("missing: %d %s", response.Code, response.Body.String())
		}
		if response := request(t, r, http.MethodGet, "/skins/"+skin.ID, "", ""); response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated: %d", response.Code)
		}
		store.SetPermissions("reader", []string{"skins.manage"})
		if response := request(t, r, http.MethodGet, "/skins/"+skin.ID, "", auth.AccessToken); response.Code != http.StatusForbidden {
			t.Fatalf("manage without read: %d", response.Code)
		}
		store.SetPermissions("reader", []string{"skins.read"})
	}
}
