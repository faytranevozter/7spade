package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// RemovePlayer validates both path params before touching the database, so the
// bad-UUID cases can be exercised without a DB connection.
func TestRemovePlayerRejectsInvalidUUIDs(t *testing.T) {
	cases := []struct {
		name    string
		roomID  string
		userID  string
		wantMsg string
	}{
		{name: "bad room id", roomID: "not-a-uuid", userID: "11111111-1111-1111-1111-111111111111", wantMsg: "Invalid room ID"},
		{name: "bad user id", roomID: "11111111-1111-1111-1111-111111111111", userID: "not-a-uuid", wantMsg: "Invalid user ID"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := RoomHandler{DB: nil}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{
				{Key: "id", Value: tc.roomID},
				{Key: "userId", Value: tc.userID},
			}
			c.Request = httptest.NewRequest(http.MethodDelete, "/internal/rooms/"+tc.roomID+"/players/"+tc.userID, nil)

			h.RemovePlayer(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Error != tc.wantMsg {
				t.Fatalf("error = %q, want %q", body.Error, tc.wantMsg)
			}
		})
	}
}
