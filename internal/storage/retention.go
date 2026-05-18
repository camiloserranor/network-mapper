package storage

import (
	"context"
	"log"
	"time"
)

const pruneInterval = 1 * time.Hour

// RetentionPruner periodically removes old snapshots and log files.
type RetentionPruner struct {
	snapshots     *SnapshotStore
	logDir        string
	retentionDays int
	maxSnapshots  int
}

// NewRetentionPruner creates a pruner that enforces age-based and count-based
// retention for topology snapshots and log files.
func NewRetentionPruner(snapshots *SnapshotStore, logDir string, retentionDays, maxSnapshots int) *RetentionPruner {
	return &RetentionPruner{
		snapshots:     snapshots,
		logDir:        logDir,
		retentionDays: retentionDays,
		maxSnapshots:  maxSnapshots,
	}
}

// Start runs the pruner: once immediately, then every hour until ctx is canceled.
func (p *RetentionPruner) Start(ctx context.Context) {
	p.prune()

	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.prune()
		}
	}
}

func (p *RetentionPruner) prune() {
	maxAge := time.Duration(p.retentionDays) * 24 * time.Hour

	snapDeleted, err := p.snapshots.PruneByAge(maxAge)
	if err != nil {
		log.Printf("[retention] Error pruning snapshots by age: %v", err)
	}

	countDeleted, err := p.snapshots.PruneByCount(p.maxSnapshots)
	if err != nil {
		log.Printf("[retention] Error pruning snapshots by count: %v", err)
	}

	logDeleted, err := PruneLogFiles(p.logDir, maxAge)
	if err != nil {
		log.Printf("[retention] Error pruning log files: %v", err)
	}

	total := snapDeleted + countDeleted + logDeleted
	if total > 0 {
		log.Printf("[retention] Cleanup complete: %d snapshot(s), %d log(s) removed", snapDeleted+countDeleted, logDeleted)
	}
}
