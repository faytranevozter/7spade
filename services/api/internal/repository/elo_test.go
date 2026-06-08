package repository

import (
	"testing"

	"github.com/google/uuid"
)

// Within a single pair, the better-ranked player gains exactly what the worse
// loses (the deltas are symmetric and net to zero) when both start equal.
func TestComputeEloDeltasPairSymmetry(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	deltas := ComputeEloDeltas([]EloPlayer{
		{UserID: a, Rating: 1200, Rank: 1},
		{UserID: b, Rating: 1200, Rank: 2},
	})
	if deltas[a] <= 0 {
		t.Fatalf("winner delta = %d, want > 0", deltas[a])
	}
	if deltas[a]+deltas[b] != 0 {
		t.Fatalf("deltas not symmetric: %d + %d != 0", deltas[a], deltas[b])
	}
	// Equal ratings, K=24: expected 0.5 each → winner +12, loser -12.
	if deltas[a] != 12 || deltas[b] != -12 {
		t.Fatalf("deltas = %d/%d, want +12/-12", deltas[a], deltas[b])
	}
}

// A tie (equal rank) between equally-rated players nets zero change for both.
func TestComputeEloDeltasTie(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	deltas := ComputeEloDeltas([]EloPlayer{
		{UserID: a, Rating: 1200, Rank: 1},
		{UserID: b, Rating: 1200, Rank: 1},
	})
	if deltas[a] != 0 || deltas[b] != 0 {
		t.Fatalf("tie deltas = %d/%d, want 0/0", deltas[a], deltas[b])
	}
}

// In a 4-player game the rank-1 finisher gains and the rank-4 finisher loses;
// the whole table's deltas sum to (approximately) zero.
func TestComputeEloDeltasFourPlayerOrdering(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	players := []EloPlayer{
		{UserID: ids[0], Rating: 1200, Rank: 1},
		{UserID: ids[1], Rating: 1200, Rank: 2},
		{UserID: ids[2], Rating: 1200, Rank: 3},
		{UserID: ids[3], Rating: 1200, Rank: 4},
	}
	deltas := ComputeEloDeltas(players)

	if deltas[ids[0]] <= 0 {
		t.Fatalf("rank 1 delta = %d, want > 0", deltas[ids[0]])
	}
	if deltas[ids[3]] >= 0 {
		t.Fatalf("rank 4 delta = %d, want < 0", deltas[ids[3]])
	}
	// Monotonic: better rank gains at least as much as a worse rank.
	if !(deltas[ids[0]] > deltas[ids[1]] && deltas[ids[1]] > deltas[ids[2]] && deltas[ids[2]] > deltas[ids[3]]) {
		t.Fatalf("deltas not strictly decreasing by rank: %d/%d/%d/%d",
			deltas[ids[0]], deltas[ids[1]], deltas[ids[2]], deltas[ids[3]])
	}
	sum := deltas[ids[0]] + deltas[ids[1]] + deltas[ids[2]] + deltas[ids[3]]
	if sum < -2 || sum > 2 {
		t.Fatalf("table sum = %d, want ~0 (rounding only)", sum)
	}
}

// Beating a higher-rated opponent gains more than beating a peer; the
// underdog's win is worth more.
func TestComputeEloDeltasUnderdogGainsMore(t *testing.T) {
	under := uuid.New()
	fav := uuid.New()
	deltas := ComputeEloDeltas([]EloPlayer{
		{UserID: under, Rating: 1000, Rank: 1},
		{UserID: fav, Rating: 1400, Rank: 2},
	})
	// expected(under) ≈ 0.09 → delta ≈ 24*(1-0.09) ≈ 22.
	if deltas[under] < 18 {
		t.Fatalf("underdog gain = %d, want a large gain (~22)", deltas[under])
	}
	if deltas[under]+deltas[fav] != 0 {
		t.Fatalf("deltas not symmetric: %d + %d", deltas[under], deltas[fav])
	}
}

// Fewer than two registered players cannot be compared, so no rating moves.
func TestComputeEloDeltasSingleton(t *testing.T) {
	a := uuid.New()
	deltas := ComputeEloDeltas([]EloPlayer{{UserID: a, Rating: 1200, Rank: 1}})
	if deltas[a] != 0 {
		t.Fatalf("singleton delta = %d, want 0", deltas[a])
	}
}
