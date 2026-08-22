package repository

import "testing"

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
