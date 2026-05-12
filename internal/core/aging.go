// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
	"context"
	"log"
	"time"

	"github.com/mohi1038/memos/internal/storage"
)

// AgingWorker scans for old, low-importance memories and transitions them
// through the lifecycle: Cold -> Archived -> Deleted.
type AgingWorker struct {
	db              *storage.PostgresStore
	qdrant          *storage.QdrantStore
	interval        time.Duration
	archiveAge      time.Duration
	deleteAge       time.Duration
	archiveMaxScore float64
	deleteMaxScore  float64
	done            chan struct{}
}

// NewAgingWorker creates a new aging pipeline worker.
func NewAgingWorker(db *storage.PostgresStore, qdrant *storage.QdrantStore) *AgingWorker {
	return &AgingWorker{
		db:              db,
		qdrant:          qdrant,
		interval:        1 * time.Hour,
		archiveAge:      30 * 24 * time.Hour, // >30 days old gets archived
		deleteAge:       90 * 24 * time.Hour, // >90 days old gets deleted
		archiveMaxScore: 0.3,                 // Only archive if importance < 0.3
		deleteMaxScore:  0.1,                 // Only delete if importance < 0.1
		done:            make(chan struct{}),
	}
}

// Start runs the aging worker in the background.
func (w *AgingWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	go func() {
		defer ticker.Stop()
		// Run initial cycle shortly after startup
		time.Sleep(10 * time.Second)
		w.runCycle(ctx)

		for {
			select {
			case <-ticker.C:
				w.runCycle(ctx)
			case <-w.done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	log.Printf("Aging worker started (interval: %v, archiveAge: %v)", w.interval, w.archiveAge)
}

// Stop gracefully shuts down the worker.
func (w *AgingWorker) Stop() {
	close(w.done)
}

func (w *AgingWorker) runCycle(ctx context.Context) {
	log.Println("[Aging] Running memory aging cycle...")

	// 1. Delete very old, very low importance memories
	w.processDeletions(ctx)

	// 2. Archive cold memories (remove from vector search, keep in Postgres)
	w.processArchivals(ctx)
}

func (w *AgingWorker) processArchivals(ctx context.Context) {
	memories, err := w.db.GetColdMemories(ctx, w.archiveAge, w.archiveMaxScore)
	if err != nil {
		log.Printf("[Aging] Failed to fetch memories for archiving: %v", err)
		return
	}

	archivedCount := 0
	for _, m := range memories {
		idStr := m.ID.String()

		// Archive in Postgres
		if err := w.db.ArchiveMemory(ctx, idStr); err != nil {
			log.Printf("[Aging] Failed to archive memory %s in DB: %v", idStr, err)
			continue
		}

		// Remove from Qdrant vector index to save compute/memory
		if w.qdrant != nil {
			if err := w.qdrant.DeleteMemory(ctx, "memories", idStr); err != nil {
				log.Printf("[Aging] Failed to delete memory %s from Qdrant: %v", idStr, err)
				// we don't abort, since it's already archived in pg
			}
		}
		archivedCount++
	}

	if archivedCount > 0 {
		log.Printf("[Aging] Successfully archived %d cold memories", archivedCount)
	}
}

func (w *AgingWorker) processDeletions(ctx context.Context) {
	// Re-use GetColdMemories but with the stricter deletion thresholds
	memories, err := w.db.GetColdMemories(ctx, w.deleteAge, w.deleteMaxScore)
	if err != nil {
		log.Printf("[Aging] Failed to fetch memories for deletion: %v", err)
		return
	}

	deletedCount := 0
	for _, m := range memories {
		idStr := m.ID.String()

		// We assume it might still be in Qdrant if it bypassed archiving
		if w.qdrant != nil {
			_ = w.qdrant.DeleteMemory(ctx, "memories", idStr)
		}

		if err := w.db.DeleteMemory(ctx, idStr); err != nil {
			log.Printf("[Aging] Failed to delete memory %s from DB: %v", idStr, err)
			continue
		}
		deletedCount++
	}

	if deletedCount > 0 {
		log.Printf("[Aging] Successfully deleted %d forgotten memories", deletedCount)
	}
}
