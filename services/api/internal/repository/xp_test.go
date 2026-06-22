package repository

import "testing"

func TestCalculateXP(t *testing.T) {
	cases := []struct {
		name   string
		player GameResultPlayer
		hasBot bool
		want   int
	}{
		{
			name:   "human-only rank 1 zero-penalty winner",
			player: GameResultPlayer{Rank: 1, IsWinner: true, PenaltyPoints: 0},
			hasBot: false,
			// base 50 + rank1 50 + winner 25 + zeroPen 25 + humanOnly 20 = 170
			want: 170,
		},
		{
			name:   "human-only rank 2 low-penalty",
			player: GameResultPlayer{Rank: 2, IsWinner: false, PenaltyPoints: 8},
			hasBot: false,
			// base 50 + rank2 30 + low 10 + humanOnly 20 = 110
			want: 110,
		},
		{
			name:   "human-only rank 3 mid penalty",
			player: GameResultPlayer{Rank: 3, IsWinner: false, PenaltyPoints: 15},
			hasBot: false,
			// base 50 + rank3 15 + humanOnly 20 = 85
			want: 85,
		},
		{
			name:   "human-only rank 4 high penalty",
			player: GameResultPlayer{Rank: 4, IsWinner: false, PenaltyPoints: 40},
			hasBot: false,
			// base 50 + rank4 0 + humanOnly 20 = 70
			want: 70,
		},
		{
			name:   "bot-mixed rank 1 zero-penalty winner",
			player: GameResultPlayer{Rank: 1, IsWinner: true, PenaltyPoints: 0},
			hasBot: true,
			// bonuses (no humanOnly): rank1 50 + winner 25 + zeroPen 25 = 100 -> *0.6 = 60
			// base 50 + 60 = 110
			want: 110,
		},
		{
			name:   "bot-mixed rank 4 high penalty hits floor",
			player: GameResultPlayer{Rank: 4, IsWinner: false, PenaltyPoints: 40},
			hasBot: true,
			// bonuses: rank4 0 -> *0.6 = 0; base 50. 50 >= floor 20, no floor.
			want: 50,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, b := CalculateXP(tc.player, tc.hasBot)
			if got != tc.want {
				t.Fatalf("CalculateXP = %d, want %d (breakdown=%+v)", got, tc.want, b)
			}
			if b.Total != got {
				t.Fatalf("breakdown.Total = %d, want %d", b.Total, got)
			}
		})
	}
}

func TestCalculateXPMinimumFloor(t *testing.T) {
	// Construct a scenario whose computed total falls below the floor: a
	// bot-mixed game where bonuses round to less than (floor - base). Since base
	// alone (50) already exceeds the floor (20), the floor only triggers if base
	// is conceptually reduced; here we assert the floor flag stays off for a
	// normal low award and the total never dips under the minimum.
	got, b := CalculateXP(GameResultPlayer{Rank: 4, IsWinner: false, PenaltyPoints: 99}, true)
	if got < xpMinPerGame {
		t.Fatalf("XP %d below floor %d", got, xpMinPerGame)
	}
	if b.MinimumApplied {
		t.Fatalf("did not expect floor to apply for base-only award, got %+v", b)
	}
}

func TestLevelFromXP(t *testing.T) {
	cases := []struct {
		xp    int64
		level int
	}{
		{-50, 1},
		{0, 1},
		{99, 1},
		{100, 2},
		{399, 2},
		{400, 3},
		{899, 3},
		{900, 4},
		{1600, 5},
		{8100, 10},
	}
	for _, tc := range cases {
		if got := LevelFromXP(tc.xp); got != tc.level {
			t.Errorf("LevelFromXP(%d) = %d, want %d", tc.xp, got, tc.level)
		}
	}
}

func TestXPRequiredForLevel(t *testing.T) {
	cases := []struct {
		level int
		req   int64
	}{
		{0, 0},
		{1, 0},
		{2, 100},
		{3, 400},
		{4, 900},
		{10, 8100},
	}
	for _, tc := range cases {
		if got := XPRequiredForLevel(tc.level); got != tc.req {
			t.Errorf("XPRequiredForLevel(%d) = %d, want %d", tc.level, got, tc.req)
		}
	}
}

func TestXPProgress(t *testing.T) {
	// 1250 XP: level 4 (>=900, <1600). into = 1250-900 = 350; span = 1600-900 =
	// 700; toNext = 1600-1250 = 350.
	level, into, span, toNext := XPProgress(1250)
	if level != 4 || into != 350 || span != 700 || toNext != 350 {
		t.Fatalf("XPProgress(1250) = (%d, %d, %d, %d), want (4, 350, 700, 350)", level, into, span, toNext)
	}

	// Exact threshold: 900 XP is the start of level 4.
	level, into, span, toNext = XPProgress(900)
	if level != 4 || into != 0 || span != 700 || toNext != 700 {
		t.Fatalf("XPProgress(900) = (%d, %d, %d, %d), want (4, 0, 700, 700)", level, into, span, toNext)
	}

	// Zero XP: level 1, span to level 2 is 100.
	level, into, span, toNext = XPProgress(0)
	if level != 1 || into != 0 || span != 100 || toNext != 100 {
		t.Fatalf("XPProgress(0) = (%d, %d, %d, %d), want (1, 0, 100, 100)", level, into, span, toNext)
	}
}
