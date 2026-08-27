package repository

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestEventRewardRequirementSupportsEveryRuleType(t *testing.T) {
	intValue := func(value int64) sql.NullInt64 { return sql.NullInt64{Int64: value, Valid: true} }
	stringValue := func(value string) sql.NullString { return sql.NullString{String: value, Valid: true} }
	tests := []struct {
		name, ruleType, description string
		minimum, streak, checkIns   sql.NullInt64
		achievementID, achievement  sql.NullString
		achievementEarned           bool
		level, login, attendance    int
		wantProgress, wantTarget    *int
		wantCompleted               bool
	}{
		{name: "event check-ins", ruleType: "event_check_in_count", description: "Check in on 3 event days", checkIns: intValue(3), attendance: 2, wantProgress: intPtr(2), wantTarget: intPtr(3)},
		{name: "minimum level", ruleType: "minimum_level", description: "Reach player level 5 during the event", minimum: intValue(5), level: 7, wantProgress: intPtr(7), wantTarget: intPtr(5), wantCompleted: true},
		{name: "login streak", ruleType: "login_streak", description: "Reach a 4-day login streak during the event", streak: intValue(4), login: 1, wantProgress: intPtr(1), wantTarget: intPtr(4)},
		{name: "achievement", ruleType: "achievement", description: "Earn the First Win achievement during the event", achievementID: stringValue("first_win"), achievement: stringValue("First Win"), achievementEarned: true, wantProgress: intPtr(1), wantTarget: intPtr(1), wantCompleted: true},
		{name: "game condition", ruleType: "game_condition", description: "Complete the Perfect Hand challenge during the event", wantCompleted: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eventRewardRequirement(tt.ruleType, map[bool]string{true: "Perfect Hand"}[tt.ruleType == "game_condition"], tt.minimum, tt.streak, tt.checkIns, tt.achievementID, tt.achievement, true, tt.ruleType == "game_condition", tt.achievementEarned, tt.level, tt.login, tt.attendance)
			if got.Description != tt.description || got.Completed != tt.wantCompleted || !equalIntPtr(got.Progress, tt.wantProgress) || !equalIntPtr(got.Target, tt.wantTarget) {
				t.Fatalf("eventRewardRequirement() = %+v", got)
			}
		})
	}
}

func TestEventRewardRequirementOmitsPersonalProgressWhenAnonymous(t *testing.T) {
	requirement := eventRewardRequirement(
		"minimum_level", "Level Five", sql.NullInt64{Int64: 5, Valid: true},
		sql.NullInt64{}, sql.NullInt64{}, sql.NullString{}, sql.NullString{},
		false, false, false, 9, 0, 0,
	)
	if requirement.Progress != nil || requirement.Target != nil || requirement.Completed {
		t.Fatalf("anonymous requirement leaked progress: %+v", requirement)
	}
	if requirement.Description != "Reach player level 5 during the event" {
		t.Fatalf("description = %q", requirement.Description)
	}
}

func TestGetEventSkinRewardsIncludesEveryEnabledRuleType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := uuid.New()
	columns := []string{"id", "skin_type", "name", "description", "asset_key", "display_order", "rule_type", "rule_name", "minimum_level", "login_streak_days", "event_check_in_count", "achievement_id", "achievement_name", "owned", "achievement_earned", "xp", "current_streak"}
	rows := sqlmock.NewRows(columns).
		AddRow(uuid.NewString(), "avatar_frame", "Check-in", "", "a", 1, "event_check_in_count", "Attend", nil, nil, 3, nil, nil, false, false, 1600, 2).
		AddRow(uuid.NewString(), "avatar_frame", "Level", "", "b", 2, "minimum_level", "Level", 5, nil, nil, nil, nil, false, false, 1600, 2).
		AddRow(uuid.NewString(), "avatar_frame", "Streak", "", "c", 3, "login_streak", "Streak", nil, 4, nil, nil, nil, false, false, 1600, 2).
		AddRow(uuid.NewString(), "avatar_frame", "Achievement", "", "d", 4, "achievement", "Achievement", nil, nil, nil, "first_win", "First Win", false, true, 1600, 2).
		AddRow(uuid.NewString(), "avatar_frame", "Challenge", "", "e", 5, "game_condition", "Perfect Hand", nil, nil, nil, nil, nil, true, false, 1600, 2)
	mock.ExpectQuery("FROM skin_unlock_rules r").WithArgs("event-id", userID, "active").WillReturnRows(rows)
	got, err := getEventSkinRewards(db, "event-id", &userID, 2, "active")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("len(rewards) = %d, want 5", len(got))
	}
	for i, ruleType := range []string{"event_check_in_count", "minimum_level", "login_streak", "achievement", "game_condition"} {
		if got[i].Requirement.Type != ruleType {
			t.Fatalf("reward %d type = %q", i, got[i].Requirement.Type)
		}
		if len(got[i].Skin.UnlockRules) != 1 || got[i].Skin.UnlockRules[0].RuleType != ruleType {
			t.Fatalf("reward %d unlock rules = %+v", i, got[i].Skin.UnlockRules)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func intPtr(value int) *int { return &value }
func equalIntPtr(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func TestEventCheckInRequirement(t *testing.T) {
	if got := eventCheckInRequirement(1); got != "Check in on 1 event day" {
		t.Fatalf("eventCheckInRequirement(1) = %q", got)
	}
	if got := eventCheckInRequirement(3); got != "Check in on 3 event days" {
		t.Fatalf("eventCheckInRequirement(3) = %q", got)
	}
}

func TestEventDayUsesAppTimezone(t *testing.T) {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 18, 30, 0, 0, time.UTC)
	if got := eventDay(now, location); got != "2026-08-26" {
		t.Fatalf("eventDay() = %q, want 2026-08-26", got)
	}
}

func TestEventStatusBoundaries(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"before", start.Add(-time.Nanosecond), "upcoming"},
		{"at start", start, "active"},
		{"before end", end.Add(-time.Nanosecond), "active"},
		{"at end", end, "ended"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventStatus(tt.now, start, end); got != tt.want {
				t.Fatalf("eventStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListEventsReturnsEnabledEventsInDiscoveryOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	columns := []string{"id", "slug", "name", "summary", "starts_at", "ends_at", "hero_asset_key", "accent_color", "reward_count"}
	rows := sqlmock.NewRows(columns).
		AddRow("active", "summer", "Summer", "Live now", now.Add(-time.Hour), now.Add(time.Hour), "summer-hero", "#d4af37", 4).
		AddRow("upcoming", "autumn", "Autumn", "Coming soon", now.Add(24*time.Hour), now.Add(48*time.Hour), nil, nil, 2).
		AddRow("ended", "spring", "Spring", "Finished", now.Add(-48*time.Hour), now.Add(-24*time.Hour), nil, nil, 1)
	mock.ExpectQuery("FROM events e").WillReturnRows(rows)

	events, err := ListEvents(db, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	for i, want := range []string{"active", "upcoming", "ended"} {
		if events[i].Status != want {
			t.Fatalf("events[%d].Status = %q, want %q", i, events[i].Status, want)
		}
	}
	if events[0].RewardCount != 4 || events[0].AppTimezone != "UTC" || !events[0].ServerTime.Equal(now) {
		t.Fatalf("active event metadata = %+v", events[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListEventsReturnsEmptySlice(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("FROM events e").WillReturnRows(sqlmock.NewRows([]string{"id", "slug", "name", "summary", "starts_at", "ends_at", "hero_asset_key", "accent_color", "reward_count"}))

	events, err := ListEvents(db, time.Now(), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if events == nil || len(events) != 0 {
		t.Fatalf("events = %#v, want non-nil empty slice", events)
	}
}
