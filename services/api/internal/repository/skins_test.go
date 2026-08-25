package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestIsSkinType(t *testing.T) {
	valid := []string{
		SkinTypeProfileBackground,
		SkinTypeAvatarFrame,
		SkinTypeDisplayPicture,
		SkinTypePlayerCardBackground,
	}
	for _, skinType := range valid {
		if !IsSkinType(skinType) {
			t.Errorf("IsSkinType(%q) = false, want true", skinType)
		}
	}
	if IsSkinType("unknown") {
		t.Error("IsSkinType(\"unknown\") = true, want false")
	}
}

func TestGetSkinCatalogIncludesServerAuthoredRequirements(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order").
		WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "unlock_requirement"}).
			AddRow("skin-1", SkinTypeAvatarFrame, "Victor", "reward", "victor.svg", 1, "Earn the First Win achievement").
			AddRow("skin-2", SkinTypeDisplayPicture, "Veteran", "reward", "veteran.svg", 2, "Reach player level 10").
			AddRow("skin-3", SkinTypePlayerCardBackground, "Rising Champion", "reward", "champion.svg", 3, "Win a completed game and Play at least 3 games and Win at least 2 games"))

	catalog, err := GetSkinCatalog(db)
	if err != nil {
		t.Fatalf("GetSkinCatalog: %v", err)
	}
	if len(catalog) != 3 || catalog[0].UnlockRequirement != "Earn the First Win achievement" || catalog[1].UnlockRequirement != "Reach player level 10" {
		t.Fatalf("catalog = %+v", catalog)
	}
	if catalog[2].UnlockRequirement != "Win a completed game and Play at least 3 games and Win at least 2 games" {
		t.Fatalf("game-condition requirement = %q", catalog[2].UnlockRequirement)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantGameConditionSkinsGrantsMatchingRulesAndSkipsInvalidOnes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectQuery("SELECT r.id, r.name, r.skin_id, c.metric, c.operator, c.value").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "skin_id", "metric", "operator", "value"}).
			AddRow("rule-1", "winner", "skin-1", "is_winner", "eq", "true").
			AddRow("rule-1", "winner", "skin-1", "penalty", "lte", "0").
			AddRow("rule-2", "games", "skin-2", "games_played", "gte", "10").
			AddRow("rule-3", "bad-metric", "skin-3", "client_claim", "eq", "true").
			AddRow("rule-4", "bad-value", "skin-4", "wins", "gte", "many").
			AddRow("rule-5", "bad-operator", "skin-5", "is_winner", "gte", "true").
			AddRow("rule-6", "winner-with-low-penalty", "skin-6", "is_winner", "eq", "true").
			AddRow("rule-6", "winner-with-low-penalty", "skin-6", "penalty", "lt", "0"))
	for _, grant := range []struct{ id, name, skin string }{{"rule-1", "winner", "skin-1"}, {"rule-2", "games", "skin-2"}} {
		mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, grant.skin, grant.id, grant.name).
			WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}).
				AddRow(grant.skin, SkinTypeAvatarFrame, grant.name, "reward", "asset.svg", 1, "game_condition:"+grant.name))
	}

	grants, err := GrantGameConditionSkins(tx, userID, achievementContext{IsWinner: true, GamesPlayed: 10})
	if err != nil {
		t.Fatalf("GrantGameConditionSkins: %v", err)
	}
	if len(grants) != 2 || grants[0].Source != "game_condition:winner" || grants[1].Source != "game_condition:games" {
		t.Fatalf("grants = %+v", grants)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantGameConditionSkinsDoesNotAnnounceExistingOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectQuery("SELECT r.id, r.name, r.skin_id, c.metric, c.operator, c.value").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "skin_id", "metric", "operator", "value"}).
			AddRow("rule-1", "winner", "skin-1", "is_winner", "eq", "true"))
	mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, "skin-1", "rule-1", "winner").
		WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}))

	grants, err := GrantGameConditionSkins(tx, userID, achievementContext{IsWinner: true})
	if err != nil {
		t.Fatalf("GrantGameConditionSkins: %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("grants = %+v, want none", grants)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantAchievementSkinsReturnsOnlyNewEnabledOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectQuery("INSERT INTO user_skins").
		WithArgs(userID, "first_win").
		WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}).
			AddRow("a0000000-0000-0000-0000-000000000010", SkinTypeAvatarFrame, "Victor Frame", "First win reward", "skins/frames/victor.svg", 50, "achievement:first_win").
			AddRow("a0000000-0000-0000-0000-000000000011", SkinTypeDisplayPicture, "Victor", "First win reward", "skins/display-pictures/victor.svg", 60, "achievement:first_win"))

	grants, err := GrantAchievementSkins(tx, userID, []string{"first_win"})
	if err != nil {
		t.Fatalf("GrantAchievementSkins: %v", err)
	}
	if len(grants) != 2 || grants[0].Source != "achievement:first_win" {
		t.Fatalf("grants = %+v", grants)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantMinimumLevelSkinsGrantsEveryEligibleThreshold(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}).
			AddRow("skin-level-2", SkinTypeAvatarFrame, "Level 2", "reward", "level-2.svg", 2, "level:2").
			AddRow("skin-level-3", SkinTypeDisplayPicture, "Level 3", "reward", "level-3.svg", 3, "level:3"))

	grants, err := GrantMinimumLevelSkins(tx, userID, 3)
	if err != nil {
		t.Fatalf("GrantMinimumLevelSkins: %v", err)
	}
	if len(grants) != 2 || grants[0].Source != "level:2" || grants[1].Source != "level:3" {
		t.Fatalf("grants = %+v", grants)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantMinimumLevelSkinsDoesNotAnnounceExistingOrIneligibleRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}))

	grants, err := GrantMinimumLevelSkins(tx, userID, 2)
	if err != nil {
		t.Fatalf("GrantMinimumLevelSkins: %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("grants = %+v, want none", grants)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
