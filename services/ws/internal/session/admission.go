package session

import "log"

// RoomAdmissions is the small room-facing admission boundary. It owns room
// state transitions; session owns socket lifetime and rejected-session delivery.
type RoomAdmissions interface {
	AdmitPlayer(string, *Claims, ID, string) error
	AdmitSpectator(string, *Claims, ID) error
}

// Admit creates the handler callback used after authentication and WebSocket
// upgrade. Keeping this orchestration in session prevents room from owning an
// HTTP-admitted connection's lifecycle.
func Admit(rooms RoomAdmissions, sessions Delivery) Serve {
	return func(roomID string, claims *Claims, sessionID ID, token string, spectator bool) {
		if spectator {
			if err := rooms.AdmitSpectator(roomID, claims, sessionID); err != nil {
				writeAdmissionError(sessions, sessionID, err)
			}
			return
		}
		if err := rooms.AdmitPlayer(roomID, claims, sessionID, token); err != nil {
			writeAdmissionError(sessions, sessionID, err)
		}
	}
}

func writeAdmissionError(sessions Delivery, sessionID ID, err error) {
	if writeErr := sessions.Send(sessionID, map[string]any{"type": "error", "message": err.Error(), "fatal": true}); writeErr != nil {
		log.Printf("write websocket join error: %v", writeErr)
	}
	sessions.Close(sessionID)
}
