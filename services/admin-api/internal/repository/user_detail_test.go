package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetUserRewardJoinsAndMissingStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT u.id, u.username, u.display_name, u.version, NULL::TEXT`).WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "version", "email", "created_at", "reason", "expires_at"}).AddRow("user", "ace", "Ace", 1, nil, now, nil, nil))
	mock.ExpectQuery(`SELECT provider FROM user_providers`).WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"provider"}).AddRow("google"))
	mock.ExpectQuery(`SELECT games_played, wins, total_penalty, xp`).WithArgs("user").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT rating_before, rating_after, rating_delta, created_at .* LIMIT 100`).WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"rating_before", "rating_after", "rating_delta", "created_at"}))
	mock.ExpectQuery(`SELECT ua.achievement_id, a.name, a.description, a.icon, ua.earned_at .* JOIN achievements a ON a.id = ua.achievement_id`).WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "icon", "earned_at"}).AddRow("win", "First win", "Win a game", "W", now)).RowsWillBeClosed()
	// Exact ownership join: no catalog fallback or disabled-revision filter; rule sources are nullable.
	mock.ExpectQuery(`SELECT s.id, s.name, s.skin_type, COALESCE\(us.source, 'unlock_rule'\), COALESCE\(us.skin_revision_id::text, ''\), COALESCE\(sr.asset_key, ''\), us.earned_at FROM user_skins us JOIN skins s ON s.id = us.skin_id LEFT JOIN skin_revisions sr ON sr.id = us.skin_revision_id AND sr.skin_id = s.id WHERE us.user_id = \$1 ORDER BY us.earned_at DESC, s.id DESC`).WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "source", "revision", "asset", "earned"}).AddRow("skin", "Old art", "avatar_frame", "admin_grant", "old-revision", "old.png", now).AddRow("rule", "Rule reward", "display_picture", "unlock_rule", "rule-revision", "rule.png", now).AddRow("missing", "Missing revision", "profile_background", "admin_grant", "missing-revision", "", now)).RowsWillBeClosed()
	mock.ExpectQuery(`SELECT g.id, g.room_id, g.finished_at, gp.penalty_points, gp.rank .* LIMIT 100`).WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"id", "room_id", "finished_at", "penalty_points", "rank"}))
	mock.ExpectQuery(`SELECT r.id, r.status, r.created_at`).WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"id", "status", "created_at"}))
	detail, err := NewPostgresStore(db, "test").GetUser(context.Background(), "user", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Stats) != 0 || detail.User.Email != "" {
		t.Fatalf("missing stats/redaction: %+v", detail)
	}
	if len(detail.Achievements) != 1 || detail.Achievements[0].Name != "First win" || detail.Achievements[0].Icon != "W" {
		t.Fatalf("achievements: %+v", detail.Achievements)
	}
	if len(detail.Skins) != 3 || detail.Skins[0].AssetKey != "old.png" || detail.Skins[0].RevisionID != "old-revision" || detail.Skins[1].Source != "unlock_rule" || detail.Skins[2].AssetKey != "" {
		t.Fatalf("skins: %+v", detail.Skins)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
