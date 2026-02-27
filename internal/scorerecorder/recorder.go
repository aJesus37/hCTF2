package scorerecorder

import (
	"context"
	"log"
	"time"

	"github.com/yourusername/hctf2/internal/database"
)

// Recorder periodically records score snapshots
type Recorder struct {
	db         *database.DB
	interval   time.Duration
	topN       int
	cancelFunc context.CancelFunc
}

// New creates a new score recorder
func New(db *database.DB, interval time.Duration, topN int) *Recorder {
	return &Recorder{
		db:       db,
		interval: interval,
		topN:     topN,
	}
}

// Start begins the background recording loop
func (r *Recorder) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	// Do initial recording immediately
	r.record(ctx)

	ticker := time.NewTicker(r.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.record(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start cleanup job (daily)
	go r.cleanupLoop(ctx)

	log.Printf("[Recorder] Started with interval=%v, topN=%d", r.interval, r.topN)
}

// Stop halts the recorder
func (r *Recorder) Stop() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

// ForceRecord triggers an immediate score recording (for admin manual trigger)
func (r *Recorder) ForceRecord() error {
	ctx := context.Background()
	r.record(ctx)
	return nil
}

func (r *Recorder) record(ctx context.Context) {
	entries, err := r.db.GetScoreboard(r.topN)
	if err != nil {
		log.Printf("[Recorder] Failed to get scoreboard: %v", err)
		return
	}

	for _, entry := range entries {
		teamID := ""
		if entry.TeamID != nil {
			teamID = *entry.TeamID
		}
		if err := r.db.InsertScoreHistory(entry.UserID, teamID, entry.Points, entry.SolveCount); err != nil {
			log.Printf("[Recorder] Failed to record score for user %s: %v", entry.UserID, err)
		}
	}

	log.Printf("[Recorder] Recorded scores for %d users", len(entries))
}

func (r *Recorder) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.db.CleanupScoreHistory(90); err != nil {
				log.Printf("[Recorder] Cleanup failed: %v", err)
			} else {
				log.Printf("[Recorder] Cleaned up old score history")
			}
		case <-ctx.Done():
			return
		}
	}
}
