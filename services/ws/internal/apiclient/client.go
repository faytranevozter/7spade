package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/faytranevozter/7spade/services/ws/internal/protocol"
)

type APIGameHistoryStore struct {
	URL    string
	Client *http.Client
	Secret string
}

type APIRoomSettingsStore struct {
	URL    string
	Client *http.Client
}

func (store *APIRoomSettingsStore) GetRoomSettings(roomID, token string) (protocol.RoomSettings, error) {
	req, err := http.NewRequest(http.MethodGet, store.URL+"/rooms/"+roomID, nil)
	if err != nil {
		return protocol.RoomSettings{}, err
	}
	// Reuse the player's JWT because the existing API room endpoint is protected
	// by normal user auth, not the internal-service secret.
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := store.Client.Do(req)
	if err != nil {
		return protocol.RoomSettings{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return protocol.RoomSettings{}, fmt.Errorf("get room settings returned status %d", resp.StatusCode)
	}
	var settings protocol.RoomSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return protocol.RoomSettings{}, err
	}
	return settings, nil
}

func (store *APIGameHistoryStore) SaveGame(result protocol.SavedGameResult) (string, []protocol.PlayerDelta, error) {
	payload, err := json.Marshal(result)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequest(http.MethodPost, store.URL, bytes.NewReader(payload))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	setInternalSecret(req, store.Secret)
	resp, err := store.Client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", nil, fmt.Errorf("save game returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var saveResp struct {
		GameID string                 `json:"game_id"`
		Deltas []protocol.PlayerDelta `json:"deltas"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&saveResp); err != nil {
		return "", nil, nil
	}
	return saveResp.GameID, saveResp.Deltas, nil
}

// setInternalSecret attaches the shared internal-API secret header when one is
// configured, so the API's /internal guard accepts the request.
func setInternalSecret(req *http.Request, secret string) {
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
}

type APIPlayerAccessChecker struct {
	URL    string
	Client *http.Client
	Secret string
}

func (checker *APIPlayerAccessChecker) CheckAccess(userID string) error {
	req, err := http.NewRequest(http.MethodGet, checker.URL+"/internal/users/"+userID+"/access", nil)
	if err != nil {
		return err
	}
	setInternalSecret(req, checker.Secret)
	response, err := checker.Client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusForbidden {
		return protocol.ErrAccessDenied
	}
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("player access returned status %d", response.StatusCode)
	}
	return nil
}

type APIRoomStatusUpdater struct {
	URL    string
	Client *http.Client
	Secret string
}

func (u *APIRoomStatusUpdater) UpdateRoomStatus(roomID, status string) error {
	payload, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/internal/rooms/%s/status", u.URL, roomID)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setInternalSecret(req, u.Secret)
	resp, err := u.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("update room status returned status %d", resp.StatusCode)
	}
	return nil
}

type APIRoomMemberRemover struct {
	URL    string
	Client *http.Client
	Secret string
}

func (r *APIRoomMemberRemover) RemoveRoomPlayer(roomID, userID string) error {
	endpoint := fmt.Sprintf("%s/internal/rooms/%s/players/%s", r.URL, roomID, userID)
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	setInternalSecret(req, r.Secret)
	resp, err := r.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("remove room player returned status %d", resp.StatusCode)
	}
	return nil
}

// KickRoomPlayer removes a player and records the kick so they can't rejoin.
func (r *APIRoomMemberRemover) KickRoomPlayer(roomID, userID string) error {
	endpoint := fmt.Sprintf("%s/internal/rooms/%s/kick/%s", r.URL, roomID, userID)
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	setInternalSecret(req, r.Secret)
	resp, err := r.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("kick room player returned status %d", resp.StatusCode)
	}
	return nil
}

type APIRoomReconciler struct {
	URL    string
	Client *http.Client
	Secret string
}

func (r *APIRoomReconciler) ReconcileRooms(activeRoomIDs []string) error {
	payload, err := json.Marshal(map[string][]string{"active_room_ids": activeRoomIDs})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/internal/rooms/reconcile", r.URL)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setInternalSecret(req, r.Secret)
	resp, err := r.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("reconcile rooms returned status %d", resp.StatusCode)
	}
	return nil
}
