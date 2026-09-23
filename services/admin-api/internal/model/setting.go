package model

import (
	"encoding/json"
	"time"
)

type FeatureSetting struct {
	Key       string          `json:"key"`
	Type      string          `json:"type"`
	Value     json.RawMessage `json:"value"`
	UpdatedAt time.Time       `json:"updated_at"`
}
