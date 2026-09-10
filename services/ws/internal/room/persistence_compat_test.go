package room

import (
	"encoding/json"

	"github.com/faytranevozter/7spade/services/ws/store"
)

// Legacy test helper: production conversion now belongs to persistence.
func toStoreSnapshot(snapshot roomSnapshot) store.RoomSnapshot {
	payload, _ := json.Marshal(DurableSnapshotFromLive(snapshot))
	var persisted store.RoomSnapshot
	_ = json.Unmarshal(payload, &persisted)
	return persisted
}

func fromStoreSnapshot(snapshot store.RoomSnapshot) roomSnapshot {
	payload, _ := json.Marshal(snapshot)
	var durable DurableSnapshot
	_ = json.Unmarshal(payload, &durable)
	return LiveSnapshotFromDurable(durable)
}
