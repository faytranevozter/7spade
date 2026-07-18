package accountcleanup

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/google/uuid"
)

// DefaultInterval is how often the API process checks for due deletions.
const DefaultInterval = 24 * time.Hour

// DefaultBatchLimit caps how many accounts one tick finalizes.
const DefaultBatchLimit = 50

// Runner periodically finalizes accounts whose deletion grace period has ended.
type Runner struct {
	DB       *sql.DB
	Interval time.Duration
	Grace    time.Duration
	Limit    int
}

// Start launches a background ticker until ctx is cancelled. Safe to call once
// at process startup. The first run is delayed by Interval (not immediate) so
// startup stays fast; call RunOnce from tests for immediate coverage.
func (r *Runner) Start(ctx context.Context) {
	interval := r.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunOnce()
			}
		}
	}()
	log.Printf("account_deletion: finalizer started interval=%s grace=%s", interval, r.grace())
}

// RunOnce finalizes all due accounts up to Limit.
func (r *Runner) RunOnce() {
	grace := r.grace()
	limit := r.Limit
	if limit <= 0 {
		limit = DefaultBatchLimit
	}
	ids, err := repository.ListUsersDueForDeletion(r.DB, grace, limit)
	if err != nil {
		log.Printf("account_deletion: list due: %v", err)
		return
	}
	for _, id := range ids {
		r.finalizeOne(id)
	}
}

func (r *Runner) finalizeOne(id uuid.UUID) {
	ok, err := repository.FinalizeAccountDeletion(r.DB, id)
	if err != nil {
		log.Printf("account_deletion: finalize user_id=%s err=%v", id, err)
		return
	}
	if ok {
		log.Printf("account_deletion: finalize user_id=%s", id)
	}
}

func (r *Runner) grace() time.Duration {
	if r.Grace > 0 {
		return r.Grace
	}
	return repository.AccountDeletionGracePeriod
}
