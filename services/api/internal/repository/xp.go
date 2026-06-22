package repository

import "math"

// XP awards and level progression.
//
// XP is lifetime-only and is granted to registered players for each saved
// (non-practice) game. The award is computed from the final per-game result via
// CalculateXP and applied inside SaveGame's transaction. Level is derived from
// total XP at read time (LevelFromXP / XPProgress) and is never stored.

// XP award constants. Tunable; changing them only affects future games (past
// player_xp_events rows preserve the breakdown that was applied at the time).
const (
	xpBaseCompletion   = 50 // every saved game
	xpRank1Bonus       = 50
	xpRank2Bonus       = 30
	xpRank3Bonus       = 15
	xpRank4Bonus       = 0
	xpWinnerBonus      = 25 // is_winner (independent of rank for shared wins)
	xpZeroPenaltyBonus = 25 // penalty == 0
	xpLowPenaltyBonus  = 10 // penalty <= xpLowPenaltyThreshold (and not zero)
	xpHumanOnlyBonus   = 20 // game had no bots
	xpLowPenaltyThreshold = 10
	xpBotMixedMultiplier  = 0.6 // applied to the bonus total when the game had a bot
	xpMinPerGame          = 20  // floor so a saved game is always worth something
)

// XPBreakdown is the per-game XP audit detail, stored as JSONB on
// player_xp_events so an award can be explained or debugged later.
type XPBreakdown struct {
	Base               int     `json:"base"`
	RankBonus          int     `json:"rank_bonus"`
	WinnerBonus        int     `json:"winner_bonus"`
	ZeroPenaltyBonus   int     `json:"zero_penalty_bonus"`
	LowPenaltyBonus    int     `json:"low_penalty_bonus"`
	HumanOnlyBonus     int     `json:"human_only_bonus"`
	BotMixedMultiplier float64 `json:"bot_mixed_multiplier"`
	MinimumApplied     bool    `json:"minimum_applied"`
	Total              int     `json:"total"`
}

// rankBonus maps a finishing rank (1..4) to its bonus. Unknown ranks award 0.
func rankBonus(rank int) int {
	switch rank {
	case 1:
		return xpRank1Bonus
	case 2:
		return xpRank2Bonus
	case 3:
		return xpRank3Bonus
	case 4:
		return xpRank4Bonus
	default:
		return 0
	}
}

// CalculateXP computes the XP delta a registered player earns for one saved
// game, plus the breakdown for the audit row. hasBot is the game-level flag
// (true when at least one seat was a bot), which both removes the human-only
// bonus and applies the bot-mixed multiplier to the bonus total.
//
// The bot-mixed multiplier scales only the bonuses, not the base completion
// award, so finishing a bot game is never worth less than playing it. A floor
// of xpMinPerGame guarantees every saved game grants something.
func CalculateXP(player GameResultPlayer, hasBot bool) (int, XPBreakdown) {
	b := XPBreakdown{
		Base:               xpBaseCompletion,
		RankBonus:          rankBonus(player.Rank),
		BotMixedMultiplier: 1.0,
	}
	if player.IsWinner {
		b.WinnerBonus = xpWinnerBonus
	}
	if player.PenaltyPoints == 0 {
		b.ZeroPenaltyBonus = xpZeroPenaltyBonus
	} else if player.PenaltyPoints <= xpLowPenaltyThreshold {
		b.LowPenaltyBonus = xpLowPenaltyBonus
	}
	if !hasBot {
		b.HumanOnlyBonus = xpHumanOnlyBonus
	}

	bonus := b.RankBonus + b.WinnerBonus + b.ZeroPenaltyBonus + b.LowPenaltyBonus + b.HumanOnlyBonus
	if hasBot {
		b.BotMixedMultiplier = xpBotMixedMultiplier
		bonus = int(math.Round(float64(bonus) * xpBotMixedMultiplier))
	}

	total := b.Base + bonus
	if total < xpMinPerGame {
		total = xpMinPerGame
		b.MinimumApplied = true
	}
	b.Total = total
	return total, b
}

// LevelFromXP derives a 1-based level from total lifetime XP using a quadratic
// curve: level n requires (n-1)^2 * 100 XP. Equivalent to
// floor(sqrt(xp/100)) + 1. Negative XP is clamped to level 1.
func LevelFromXP(xp int64) int {
	if xp <= 0 {
		return 1
	}
	return int(math.Floor(math.Sqrt(float64(xp)/100.0))) + 1
}

// XPRequiredForLevel returns the cumulative XP needed to reach a given level.
// Level 1 (and below) requires 0.
func XPRequiredForLevel(level int) int64 {
	if level <= 1 {
		return 0
	}
	n := int64(level - 1)
	return n * n * 100
}

// XPProgress decomposes total XP into the current level and the progress within
// it, suitable for a progress bar:
//   - level:        current 1-based level
//   - intoLevel:    XP earned past the current level's threshold
//   - forNextLevel: XP span between this level and the next
//   - toNextLevel:  XP still needed to reach the next level
func XPProgress(xp int64) (level int, intoLevel, forNextLevel, toNextLevel int64) {
	if xp < 0 {
		xp = 0
	}
	level = LevelFromXP(xp)
	currentFloor := XPRequiredForLevel(level)
	nextFloor := XPRequiredForLevel(level + 1)
	intoLevel = xp - currentFloor
	forNextLevel = nextFloor - currentFloor
	toNextLevel = nextFloor - xp
	if toNextLevel < 0 {
		toNextLevel = 0
	}
	return level, intoLevel, forNextLevel, toNextLevel
}
