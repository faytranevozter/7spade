package handler

import (
	"context"
	"sort"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
)

func (s *MemoryStore) ListFeatureSettings(_ context.Context) ([]model.FeatureSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := make([]model.FeatureSetting, 0, len(s.featureSettings))
	for key, enabled := range s.featureSettings {
		settings = append(settings, model.FeatureSetting{Key: key, Enabled: enabled})
	}
	sort.Slice(settings, func(i, j int) bool { return settings[i].Key < settings[j].Key })
	return settings, nil
}

func (s *MemoryStore) GetFeatureSetting(_ context.Context, key string) (model.FeatureSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enabled, ok := s.featureSettings[key]
	if !ok {
		return model.FeatureSetting{}, ErrNotFound
	}
	return model.FeatureSetting{Key: key, Enabled: enabled}, nil
}

func (s *MemoryStore) UpdateFeatureSetting(_ context.Context, key string, enabled bool, audit AuditEvent) (model.FeatureSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, ok := s.featureSettings[key]
	if !ok {
		return model.FeatureSetting{}, ErrNotFound
	}
	s.featureSettings[key] = enabled
	audit.BeforeState = featureSettingState(before)
	audit.AfterState = featureSettingState(enabled)
	s.audits = append(s.audits, audit)
	return model.FeatureSetting{Key: key, Enabled: enabled}, nil
}
