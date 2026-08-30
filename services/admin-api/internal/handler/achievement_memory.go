package handler

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
)

func (s *MemoryStore) SetAchievements(achievements ...model.Achievement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, achievement := range achievements {
		s.achievements[achievement.ID] = achievement
	}
}

func (s *MemoryStore) ListAchievements(context.Context) ([]model.Achievement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Achievement, 0, len(s.achievements))
	for _, achievement := range s.achievements {
		out = append(out, achievement)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayOrder < out[j].DisplayOrder })
	return out, nil
}

func (s *MemoryStore) UpdateAchievement(_ context.Context, id string, next model.Achievement, event AuditEvent) (model.Achievement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.achievements[id]
	if !ok {
		return model.Achievement{}, ErrNotFound
	}
	if current.RulesLocked && !sameAchievementRules(current.Rules, next.Rules) {
		return model.Achievement{}, ErrConflict
	}
	next.ID = id
	next.RulesLocked = current.RulesLocked
	event.BeforeState, _ = json.Marshal(current)
	event.AfterState, _ = json.Marshal(next)
	s.achievements[id] = next
	s.audits = append(s.audits, event)
	return next, nil
}

func sameAchievementRules(left, right []model.AchievementRule) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (s *MemoryStore) ChangeAchievementEntitlement(_ context.Context, userID, achievementID, action, reason, key string, event AuditEvent) (model.AchievementEntitlementEvent, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.achievementIdempotency[key]; ok {
		return existing, true, nil
	}
	if _, ok := s.users[userID]; !ok {
		return model.AchievementEntitlementEvent{}, false, ErrNotFound
	}
	achievement, ok := s.achievements[achievementID]
	if !ok {
		return model.AchievementEntitlementEvent{}, false, ErrNotFound
	}
	entitlementKey := userID + ":" + achievementID
	if action == "grant" {
		if s.achievementEntitlements[entitlementKey] {
			return model.AchievementEntitlementEvent{}, false, ErrConflict
		}
		s.achievementEntitlements[entitlementKey] = true
	} else {
		if !s.achievementEntitlements[entitlementKey] {
			return model.AchievementEntitlementEvent{}, false, ErrConflict
		}
		delete(s.achievementEntitlements, entitlementKey)
	}
	achievement.RulesLocked = true
	s.achievements[achievementID] = achievement
	result := model.AchievementEntitlementEvent{ID: uuid.NewString(), UserID: userID, AchievementID: achievementID, Action: action, Reason: reason, IdempotencyKey: key, AdminID: event.AdminID, OccurredAt: time.Now()}
	s.achievementEntitlementEvents = append(s.achievementEntitlementEvents, result)
	s.achievementIdempotency[key] = result
	if action == "grant" {
		event.BeforeState, event.AfterState = []byte(`{"entitled":false}`), []byte(`{"entitled":true}`)
	} else {
		event.BeforeState, event.AfterState = []byte(`{"entitled":true}`), []byte(`{"entitled":false}`)
	}
	s.audits = append(s.audits, event)
	return result, false, nil
}

func (s *MemoryStore) AchievementEntitlementEvents() []model.AchievementEntitlementEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.AchievementEntitlementEvent(nil), s.achievementEntitlementEvents...)
}
