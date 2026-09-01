package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
)

const eventColumns = `id::text,slug,name,summary,description,starts_at,ends_at,hero_asset_key,accent_color,reward_config,lifecycle_state,revision,resource_version,published_at,archived_at`

type eventScanner interface{ Scan(...any) error }

func scanEvent(row eventScanner) (model.Event, error) {
	var event model.Event
	var reward []byte
	err := row.Scan(&event.ID, &event.Slug, &event.Name, &event.Summary, &event.Description, &event.StartsAt, &event.EndsAt, &event.HeroAssetKey, &event.AccentColor, &reward, &event.State, &event.Revision, &event.Version, &event.PublishedAt, &event.ArchivedAt)
	event.RewardConfig = reward
	return event, err
}
func (s *PostgresStore) ListEvents(ctx context.Context) ([]model.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+eventColumns+` FROM events ORDER BY starts_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []model.Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
func (s *PostgresStore) GetEvent(ctx context.Context, id string) (model.Event, error) {
	event, err := scanEvent(s.db.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM events WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Event{}, ErrNotFound
	}
	return event, err
}
func (s *PostgresStore) CreateEvent(ctx context.Context, event model.Event, audit AuditEvent) (model.Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Event{}, err
	}
	defer tx.Rollback()
	event.State = model.EventDraft
	event.Revision, event.Version = 1, 1
	created, err := scanEvent(tx.QueryRowContext(ctx, `INSERT INTO events(slug,name,summary,description,starts_at,ends_at,hero_asset_key,accent_color,reward_config,enabled,lifecycle_state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,FALSE,'draft') ON CONFLICT(slug) DO NOTHING RETURNING `+eventColumns, event.Slug, event.Name, event.Summary, event.Description, event.StartsAt, event.EndsAt, event.HeroAssetKey, event.AccentColor, event.RewardConfig))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Event{}, ErrConflict
	}
	if err != nil {
		return model.Event{}, err
	}
	audit.ResourceID = created.ID
	audit.AfterState, _ = json.Marshal(created)
	if err = appendAudit(ctx, tx, audit); err != nil {
		return model.Event{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Event{}, err
	}
	return created, nil
}
func (s *PostgresStore) UpdateEvent(ctx context.Context, id string, version int, next model.Event, audit AuditEvent) (model.Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Event{}, err
	}
	defer tx.Rollback()
	current, err := scanEvent(tx.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM events WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Event{}, ErrNotFound
	}
	if err != nil {
		return model.Event{}, err
	}
	if current.Version != version {
		return model.Event{}, ErrConflict
	}
	revision, state := current.Revision, current.State
	publishedAt := current.PublishedAt
	if current.State == model.EventPublished {
		revision++
		state = model.EventDraft
		publishedAt = nil
	}
	updated, err := scanEvent(tx.QueryRowContext(ctx, `UPDATE events SET slug=$2,name=$3,summary=$4,description=$5,starts_at=$6,ends_at=$7,hero_asset_key=$8,accent_color=$9,reward_config=$10,lifecycle_state=$11,revision=$12,resource_version=resource_version+1,published_at=$13,archived_at=$14,enabled=FALSE,updated_at=NOW() WHERE id=$1 RETURNING `+eventColumns, id, next.Slug, next.Name, next.Summary, next.Description, next.StartsAt, next.EndsAt, next.HeroAssetKey, next.AccentColor, next.RewardConfig, state, revision, publishedAt, current.ArchivedAt))
	if err != nil {
		return model.Event{}, err
	}
	audit.BeforeState, _ = json.Marshal(current)
	audit.AfterState, _ = json.Marshal(updated)
	if err = appendAudit(ctx, tx, audit); err != nil {
		return model.Event{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Event{}, err
	}
	return updated, nil
}
func (s *PostgresStore) TransitionEvent(ctx context.Context, id string, version int, state string, audit AuditEvent) (model.Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Event{}, err
	}
	defer tx.Rollback()
	current, err := scanEvent(tx.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM events WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Event{}, ErrNotFound
	}
	if err != nil {
		return model.Event{}, err
	}
	if current.Version != version {
		return model.Event{}, ErrConflict
	}
	now := time.Now().UTC()
	valid := state == model.EventScheduled && current.State == model.EventDraft && now.Before(current.StartsAt) || state == model.EventPublished && (current.State == model.EventDraft || current.State == model.EventScheduled) && now.Before(current.EndsAt) || state == model.EventArchived && current.State == model.EventPublished
	if !valid {
		return model.Event{}, ErrConflict
	}
	var publishedAt, archivedAt *time.Time
	publishedAt = current.PublishedAt
	if state == model.EventPublished {
		publishedAt = &now
	}
	if state == model.EventArchived {
		archivedAt = &now
	}
	updated, err := scanEvent(tx.QueryRowContext(ctx, `UPDATE events SET lifecycle_state=$2,resource_version=resource_version+1,published_at=$3,archived_at=$4,enabled=($2='published'),updated_at=NOW() WHERE id=$1 RETURNING `+eventColumns, id, state, publishedAt, archivedAt))
	if err != nil {
		return model.Event{}, err
	}
	if state == model.EventPublished {
		_, err = tx.ExecContext(ctx, `INSERT INTO event_versions(event_id,revision,slug,name,summary,description,starts_at,ends_at,hero_asset_key,accent_color,reward_config,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, updated.ID, updated.Revision, updated.Slug, updated.Name, updated.Summary, updated.Description, updated.StartsAt, updated.EndsAt, updated.HeroAssetKey, updated.AccentColor, updated.RewardConfig, now)
		if err != nil {
			return model.Event{}, err
		}
	}
	audit.BeforeState, _ = json.Marshal(current)
	audit.AfterState, _ = json.Marshal(updated)
	if err = appendAudit(ctx, tx, audit); err != nil {
		return model.Event{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Event{}, err
	}
	return updated, nil
}
