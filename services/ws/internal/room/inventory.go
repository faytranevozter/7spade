package room

// ActiveRoomIDs snapshots every room currently held in memory.
func (server *Manager) ActiveRoomIDs() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	ids := make([]string, 0, len(server.rooms))
	for id := range server.rooms {
		ids = append(ids, id)
	}
	return ids
}

// OwnedRoomIDs snapshots rooms this replica can authoritatively drive.
func (server *Manager) OwnedRoomIDs() []string {
	server.mu.Lock()
	rooms := make([]*room, 0, len(server.rooms))
	for _, r := range server.rooms {
		rooms = append(rooms, r)
	}
	server.mu.Unlock()
	ids := make([]string, 0, len(rooms))
	for _, r := range rooms {
		if r.isOwnerOrSolo() {
			ids = append(ids, r.id)
		}
	}
	return ids
}
