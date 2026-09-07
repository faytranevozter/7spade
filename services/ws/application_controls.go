package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	controlNewGameStarts   = "new_game_starts"
	controlSpectatorAccess = "spectator_access"
	controlEmotes          = "emotes"

	applicationControlsRefreshInterval = 5 * time.Second
	applicationControlsMaxStale        = 30 * time.Second
)

var requiredWSControls = []string{
	controlNewGameStarts,
	controlSpectatorAccess,
	controlEmotes,
}

// applicationControlsCache retains the last complete API response. Controlled
// operations fail closed until the first success and after that response ages
// beyond maxStale; refresh failures within that window use the last good value.
type applicationControlsCache struct {
	url      string
	secret   string
	client   *http.Client
	maxStale time.Duration
	now      func() time.Time

	mu          sync.RWMutex
	values      map[string]bool
	lastSuccess time.Time
}

func newApplicationControlsCache(apiURL, secret string) *applicationControlsCache {
	return &applicationControlsCache{
		url:      apiURL + "/internal/application-controls",
		secret:   secret,
		client:   &http.Client{Timeout: 5 * time.Second},
		maxStale: applicationControlsMaxStale,
		now:      time.Now,
	}
}

func (c *applicationControlsCache) Enabled(key string) bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastSuccess.IsZero() || c.now().Sub(c.lastSuccess) > c.maxStale {
		return false
	}
	return c.values[key]
}

func (c *applicationControlsCache) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	setInternalSecret(req, c.secret)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch application controls: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("fetch application controls returned status %d", resp.StatusCode)
	}
	var values map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&values); err != nil {
		return fmt.Errorf("decode application controls: %w", err)
	}
	for _, key := range requiredWSControls {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("application controls response missing %q", key)
		}
	}
	c.mu.Lock()
	c.values = values
	c.lastSuccess = c.now()
	c.mu.Unlock()
	return nil
}

func (c *applicationControlsCache) Start(ctx context.Context, interval time.Duration) {
	if c == nil {
		return
	}
	if interval <= 0 {
		interval = applicationControlsRefreshInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.Refresh(ctx); err != nil {
					log.Printf("refresh application controls: %v", err)
				}
			}
		}
	}()
}

type staticApplicationControls map[string]bool

func (c staticApplicationControls) Enabled(key string) bool { return c[key] }
