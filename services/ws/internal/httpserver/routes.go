package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

const InspectionSecretHeader = "X-WS-Inspection-Secret"

// Inspector returns an immutable inspection view and the existing response
// status. Authentication and encoding are owned by this HTTP adapter.
type Inspector func(context.Context, string, bool) (any, int)

func Routes(checks map[string]DependencyCheck, websocket http.Handler, secret string, inspect Inspector) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", HealthHandler("ws", checks))
	mux.Handle("GET /ws", websocket)
	mux.HandleFunc("GET /internal/rooms/{roomID}", InspectionHandler(secret, inspect, false))
	mux.HandleFunc("GET /internal/rooms/{roomID}/hidden-state", InspectionHandler(secret, inspect, true))
	return mux
}

func InspectionHandler(secret string, inspect Inspector, hidden bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusUnauthorized
		var value any = map[string]string{"error": "unauthorized"}
		if secret != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get(InspectionSecretHeader)), []byte(secret)) == 1 {
			value, status = inspect(r.Context(), r.PathValue("roomID"), hidden)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
}
