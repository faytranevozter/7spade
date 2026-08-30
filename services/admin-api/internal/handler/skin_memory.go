package handler

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
)

func (s *MemoryStore) SetSkins(skins ...Skin) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, skin := range skins {
		s.skins[skin.ID] = skin
	}
}
func (s *MemoryStore) ListSkins(context.Context) ([]Skin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Skin, 0, len(s.skins))
	for _, skin := range s.skins {
		out = append(out, skin)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayOrder < out[j].DisplayOrder })
	return out, nil
}

func (s *MemoryStore) SkinExists(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.skins[id]
	return ok, nil
}
func (s *MemoryStore) CreateSkin(_ context.Context, skin Skin, event AuditEvent) (Skin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	skin.ID = uuid.NewString()
	skin.AssetKey = ""
	skin.IsStarter = false
	skin.Enabled = false
	skin.CatalogVisible = false
	s.skins[skin.ID] = skin
	event.ResourceID = skin.ID
	s.audits = append(s.audits, event)
	return skin, nil
}

func (s *MemoryStore) UpdateSkin(_ context.Context, id string, next Skin, event AuditEvent) (Skin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.skins[id]
	if !ok {
		return Skin{}, ErrNotFound
	}
	if current.UnlockRulesLocked && next.UnlockRules != nil {
		return Skin{}, ErrConflict
	}
	current.Name = next.Name
	current.Description = next.Description
	current.DisplayOrder = next.DisplayOrder
	current.Enabled = next.Enabled
	current.CatalogVisible = next.CatalogVisible
	if next.UnlockRules != nil {
		current.UnlockRules = next.UnlockRules
	}
	s.skins[id] = current
	s.audits = append(s.audits, event)
	return current, nil
}
func (s *MemoryStore) PublishSkinRevision(_ context.Context, id, key, contentType string, event AuditEvent) (SkinRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	skin, ok := s.skins[id]
	if !ok {
		return SkinRevision{}, ErrNotFound
	}
	revision := SkinRevision{ID: uuid.NewString(), SkinID: id, Version: len(skin.Revisions) + 1, AssetKey: key, ContentType: contentType, Enabled: true, PublishedAt: time.Now()}
	skin.Revisions = append(skin.Revisions, revision)
	skin.AssetKey = key
	s.skins[id] = skin
	s.audits = append(s.audits, event)
	return revision, nil
}
func (s *MemoryStore) DisableSkinRevision(_ context.Context, skinID, revisionID string, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	skin, ok := s.skins[skinID]
	if !ok {
		return ErrNotFound
	}
	for i := range skin.Revisions {
		if skin.Revisions[i].ID == revisionID {
			now := time.Now()
			skin.Revisions[i].Enabled = false
			skin.Revisions[i].DisabledAt = &now
			if skin.AssetKey == skin.Revisions[i].AssetKey {
				skin.AssetKey = ""
				skin.Enabled = false
				skin.CatalogVisible = false
			}
			s.skins[skinID] = skin
			s.audits = append(s.audits, event)
			return nil
		}
	}
	return ErrNotFound
}
func (s *MemoryStore) ChangeSkinEntitlement(_ context.Context, userID, skinID, revisionID, action, reason string, event AuditEvent) (SkinEntitlementEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	skin, ok := s.skins[skinID]
	if !ok {
		return SkinEntitlementEvent{}, ErrNotFound
	}
	found := false
	for _, r := range skin.Revisions {
		if r.ID == revisionID && (action != "grant" || r.Enabled) {
			found = true
		}
	}
	if _, userExists := s.users[userID]; !userExists || !found {
		return SkinEntitlementEvent{}, ErrNotFound
	}
	key := userID + ":" + skinID
	if action == "grant" {
		if s.entitlements[key] != "" {
			return SkinEntitlementEvent{}, ErrConflict
		}
		s.entitlements[key] = revisionID
	} else {
		if s.entitlements[key] == "" || s.entitlements[key] != revisionID {
			return SkinEntitlementEvent{}, ErrConflict
		}
		delete(s.entitlements, key)
	}
	result := SkinEntitlementEvent{ID: uuid.NewString(), UserID: userID, SkinID: skinID, RevisionID: revisionID, Action: action, Reason: reason, AdminID: event.AdminID, OccurredAt: time.Now()}
	s.entitlementEvents = append(s.entitlementEvents, result)
	skin.UnlockRulesLocked = true
	s.skins[skinID] = skin
	s.audits = append(s.audits, event)
	return result, nil
}
func (s *MemoryStore) EntitlementEvents() []SkinEntitlementEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SkinEntitlementEvent(nil), s.entitlementEvents...)
}
