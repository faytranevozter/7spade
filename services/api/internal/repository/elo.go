package repository

import (
	"math"

	"github.com/google/uuid"
)

// DefaultRating is the starting ELO assigned to every registered player. It
// matches the column default in migrations 015/016 so a player's first rated
// game starts from the same baseline whether or not a row exists yet.
const DefaultRating = 1200

// eloKFactor is the standard chess-style K-factor scaling how much a single
// game can move a rating. A modest value keeps a 4-player free-for-all (up to
// 3 pairwise comparisons per player) from swinging too hard per game.
const eloKFactor = 24

// EloPlayer is one registered participant's pre-game rating and finishing rank
// (1 = best). Guests/bots are excluded by the caller and never appear here.
type EloPlayer struct {
	UserID uuid.UUID
	Rating int
	Rank   int
}

// ComputeEloDeltas adapts 1v1 ELO to a multiplayer free-for-all via pairwise
// expansion: every pair of players is scored as a head-to-head where the
// better-ranked player "wins" (score 1), the worse-ranked loses (score 0), and
// equal ranks tie (0.5 each). Each player's deltas across all opponents are
// summed and returned as a single integer adjustment.
//
//	expected = 1 / (1 + 10^((opp - me)/400))
//	delta    = K * (score - expected)   summed over opponents
//
// The result is order-independent and symmetric within a pair (what the winner
// gains over an opponent, that opponent loses). Fewer than two players yields
// no deltas (nothing to compare against). Deltas are rounded to the nearest
// integer; a player can have a zero net delta (e.g. mid-pack splits).
func ComputeEloDeltas(players []EloPlayer) map[uuid.UUID]int {
	deltas := make(map[uuid.UUID]int, len(players))
	if len(players) < 2 {
		for _, p := range players {
			deltas[p.UserID] = 0
		}
		return deltas
	}

	raw := make(map[uuid.UUID]float64, len(players))
	for i := range players {
		a := players[i]
		var sum float64
		for j := range players {
			if i == j {
				continue
			}
			b := players[j]
			expected := 1.0 / (1.0 + math.Pow(10, float64(b.Rating-a.Rating)/400.0))
			score := pairScore(a.Rank, b.Rank)
			sum += eloKFactor * (score - expected)
		}
		raw[a.UserID] = sum
	}

	for id, v := range raw {
		deltas[id] = int(math.Round(v))
	}
	return deltas
}

// pairScore is player a's outcome against player b given finishing ranks: 1 for
// a better (lower) rank, 0 for a worse rank, 0.5 for a tie.
func pairScore(rankA, rankB int) float64 {
	switch {
	case rankA < rankB:
		return 1.0
	case rankA > rankB:
		return 0.0
	default:
		return 0.5
	}
}
