package handler

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
)

func (s *MemoryStore) ListFeatureSettings(_ context.Context) ([]model.FeatureSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := make([]model.FeatureSetting, 0, len(s.featureSettings))
	for _, setting := range s.featureSettings {
		settings = append(settings, setting)
	}
	sort.Slice(settings, func(i, j int) bool { return settings[i].Key < settings[j].Key })
	return settings, nil
}

func (s *MemoryStore) GetFeatureSetting(_ context.Context, key string) (model.FeatureSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	setting, ok := s.featureSettings[key]
	if !ok {
		return model.FeatureSetting{}, ErrNotFound
	}
	return setting, nil
}

func (s *MemoryStore) UpdateFeatureSetting(_ context.Context, key string, value json.RawMessage, audit AuditEvent) (model.FeatureSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, ok := s.featureSettings[key]
	if !ok {
		return model.FeatureSetting{}, ErrNotFound
	}
	after := before
	after.Value = append(json.RawMessage(nil), value...)
	after.UpdatedAt = time.Now()
	s.featureSettings[key] = after
	audit.BeforeState = featureSettingState(before)
	audit.AfterState = featureSettingState(after)
	s.audits = append(s.audits, audit)
	return after, nil
}
