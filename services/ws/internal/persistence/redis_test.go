package persistence

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/faytranevozter/7spade/services/ws/store"
	"github.com/redis/go-redis/v9"
)

func TestRedisLoadRestoresLegacyJSONSnapshot(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	// This is a pre-extension room payload: it deliberately omits custom mode,
	// replay, version, result, and delta fields added by newer deployments.
	legacy := `{"state":{"hands":[[],[],[],[]],"face_down":[[],[],[],[]],"board":{},"closed":{},"current_player":0},"players":[],"phase":0,"started":false}`
	if err := mr.Set(store.StateKey("legacy"), legacy); err != nil {
		t.Fatal(err)
	}
	adapter := NewRedis(store.New(client, time.Hour))
	if _, ok := adapter.LoadRoom("legacy"); !ok {
		t.Fatal("legacy Redis payload was not restored")
	}
}
