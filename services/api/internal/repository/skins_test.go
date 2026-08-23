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
