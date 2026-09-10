package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestApplicationControlsCacheRefreshAndStaleness(t *testing.T) {
	var requestedSecret string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedSecret = r.Header.Get("X-Internal-Secret")
		_ = json.NewEncoder(w).Encode(map[string]bool{
			controlNewGameStarts:   true,
			controlSpectatorAccess: false,
			controlEmotes:          true,
		})
	}))
	defer api.Close()

	cache := NewApplicationControlsCache(api.URL, "shared-secret")
	now := time.Now()
	cache.now = func() time.Time { return now }
	cache.maxStale = time.Minute
	if cache.Enabled(controlNewGameStarts) {
		t.Fatal("cache must fail closed before its first successful refresh")
	}
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requestedSecret != "shared-secret" {
		t.Fatalf("internal secret = %q", requestedSecret)
	}
	if !cache.Enabled(controlNewGameStarts) || cache.Enabled(controlSpectatorAccess) || !cache.Enabled(controlEmotes) {
		t.Fatal("cache did not retain the fetched controls")
	}
	now = now.Add(time.Minute + time.Nanosecond)
	if cache.Enabled(controlNewGameStarts) || cache.Enabled(controlEmotes) {
		t.Fatal("stale controls must fail closed")
	}
}

func TestApplicationControlsCacheRejectsIncompleteResponse(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]bool{controlEmotes: true})
	}))
	defer api.Close()
	cache := NewApplicationControlsCache(api.URL, "secret")
	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("expected incomplete response to fail")
	}
	if cache.Enabled(controlEmotes) {
		t.Fatal("incomplete response must not make the cache available")
	}
}
