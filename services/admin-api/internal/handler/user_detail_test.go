package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestUserDetailRewardsResolveAssetsWithoutMutatingStoreOrRedaction(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"users.read"}})
	id := "00000000-0000-0000-0000-000000000001"
	store.SetUsers(UserDetail{User: User{ID: id, Email: "private@example.com"}, Skins: []model.UserSkin{{ID: "skin", Name: "Owned art", RevisionID: "old-revision", AssetKey: "old.png", EarnedAt: time.Now()}}, Achievements: []model.UserAchievement{{AchievementID: "win", Name: "First win", Description: "Win once", Icon: "W"}}})
	for _, storage := range []StorageSigner{&stubSkinSigner{}, nil} {
		h := NewAdminHandler(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, Dependencies{Storage: storage})
		r := gin.New()
		r.POST("/auth/login", h.Login)
		r.GET("/users/:id", h.RequireAuth, h.RequirePermission("users.read"), h.GetUser)
		login := request(t, r, http.MethodPost, "/auth/login", `{"email":"reader@example.com","password":"password"}`, "")
		var auth AuthResponse
		if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
			t.Fatal(err)
		}
		response := request(t, r, http.MethodGet, "/users/"+id, "", auth.AccessToken)
		var detail UserDetail
		if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "private@example.com") || strings.Contains(response.Body.String(), "asset_key") {
			t.Fatalf("redacted detail: %d %s", response.Code, response.Body.String())
		}
		wantURL := ""
		if storage != nil {
			wantURL = storage.PublicURL("old.png")
		}
		if len(detail.Skins) != 1 || detail.Skins[0].AssetURL != wantURL || detail.Skins[0].RevisionID != "old-revision" || detail.Achievements[0].Name != "First win" {
			t.Fatalf("rewards: %+v", detail)
		}
		saved, err := store.GetUser(context.Background(), id, true)
		if err != nil || saved.User.Email != "private@example.com" || saved.Skins[0].AssetURL != "" {
			t.Fatalf("store mutated: %+v %v", saved, err)
		}
	}
}
