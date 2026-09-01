package handler

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
)

func (s *MemoryStore) ListEvents(context.Context) ([]model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Event, 0, len(s.events))
	for _, event := range s.events {
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out, nil
}
func (s *MemoryStore) GetEvent(_ context.Context, id string) (model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	if !ok {
		return model.Event{}, ErrNotFound
	}
	return event, nil
}
func (s *MemoryStore) CreateEvent(_ context.Context, event model.Event, audit AuditEvent) (model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.events {
		if existing.Slug == event.Slug {
			return model.Event{}, ErrConflict
		}
	}
	event.ID = uuid.NewString()
	event.State = model.EventDraft
	event.Revision = 1
	event.Version = 1
	audit.ID = uuid.NewString()
	audit.ResourceID = event.ID
	audit.AfterState, _ = json.Marshal(event)
	s.events[event.ID] = event
	s.audits = append(s.audits, audit)
	return event, nil
}
func (s *MemoryStore) UpdateEvent(_ context.Context, id string, version int, next model.Event, audit AuditEvent) (model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.events[id]
	if !ok {
		return model.Event{}, ErrNotFound
	}
	if current.Version != version {
		return model.Event{}, ErrConflict
	}
	for otherID, existing := range s.events {
		if otherID != id && existing.Slug == next.Slug {
			return model.Event{}, ErrConflict
		}
	}
	next.ID = id
	next.Version = current.Version + 1
	next.Revision = current.Revision
	next.State = current.State
	next.PublishedAt = current.PublishedAt
	next.ArchivedAt = current.ArchivedAt
	if current.State == model.EventPublished {
		next.Revision++
		next.State = model.EventDraft
		next.PublishedAt = nil
	}
	audit.ID = uuid.NewString()
	audit.BeforeState, _ = json.Marshal(current)
	audit.AfterState, _ = json.Marshal(next)
	s.events[id] = next
	s.audits = append(s.audits, audit)
	return next, nil
}
func (s *MemoryStore) TransitionEvent(_ context.Context, id string, version int, state string, audit AuditEvent) (model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.events[id]
	if !ok {
		return model.Event{}, ErrNotFound
	}
	if current.Version != version {
		return model.Event{}, ErrConflict
	}
	now := time.Now().UTC()
	valid := state == model.EventScheduled && current.State == model.EventDraft && now.Before(current.StartsAt) || state == model.EventPublished && (current.State == model.EventDraft || current.State == model.EventScheduled) && now.Before(current.EndsAt) || state == model.EventArchived && current.State == model.EventPublished
	if !valid {
		return model.Event{}, ErrConflict
	}
	next := current
	next.State = state
	next.Version++
	if state == model.EventPublished {
		next.PublishedAt = &now
	}
	if state == model.EventArchived {
		next.ArchivedAt = &now
	}
	audit.ID = uuid.NewString()
	audit.BeforeState, _ = json.Marshal(current)
	audit.AfterState, _ = json.Marshal(next)
	s.events[id] = next
	s.audits = append(s.audits, audit)
	return next, nil
}
