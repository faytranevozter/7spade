package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WSAdminClient struct {
	baseURL, secret string
	client          *http.Client
}

func NewWSAdminClient(baseURL, secret string) *WSAdminClient {
	return &WSAdminClient{baseURL: strings.TrimRight(baseURL, "/"), secret: secret, client: &http.Client{Timeout: 2 * time.Second}}
}
func (c *WSAdminClient) RoomSummary(ctx context.Context, roomID string) (LiveRoomSummary, error) {
	if c.baseURL == "" || c.secret == "" {
		return LiveRoomSummary{}, fmt.Errorf("ws admin not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/rooms/"+roomID, nil)
	if err != nil {
		return LiveRoomSummary{}, err
	}
	req.Header.Set("X-WS-Inspection-Secret", c.secret)
	response, err := c.client.Do(req)
	if err != nil {
		return LiveRoomSummary{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return LiveRoomSummary{}, fmt.Errorf("ws admin status %d", response.StatusCode)
	}
	var responseBody struct {
		Phase         string           `json:"phase"`
		Players       []LiveRoomPlayer `json:"players"`
		TurnDeadline  *time.Time       `json:"turn_deadline"`
		SnapshotAgeMS int64            `json:"snapshot_age_ms"`
		StateVersion  int64            `json:"state_version"`
		Owner         struct {
			Role         string `json:"role"`
			ReplicaID    string `json:"replica_id"`
			FencingToken int64  `json:"fencing_token"`
		} `json:"owner"`
	}
	if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
		return LiveRoomSummary{}, err
	}
	return LiveRoomSummary{
		Role:               responseBody.Owner.Role,
		Phase:              responseBody.Phase,
		Players:            responseBody.Players,
		TurnDeadline:       responseBody.TurnDeadline,
		SnapshotAgeSeconds: float64(responseBody.SnapshotAgeMS) / 1000,
		StateVersion:       responseBody.StateVersion,
		OwnerID:            responseBody.Owner.ReplicaID,
		FenceToken:         responseBody.Owner.FencingToken,
	}, nil
}
