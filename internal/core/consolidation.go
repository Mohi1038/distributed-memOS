// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mohi1038/memos/internal/storage"
)

// Summarizer defines the interface for consolidating multiple memories.
type Summarizer interface {
	Summarize(ctx context.Context, memories []string) (string, error)
}

// MemoryConsolidator handles the periodic clustering and merging of episodic memories.
type MemoryConsolidator struct {
	db         *storage.PostgresStore
	qdrant     *storage.QdrantStore
	summarizer Summarizer
}

func NewMemoryConsolidator(db *storage.PostgresStore, qdrant *storage.QdrantStore, summarizer Summarizer) *MemoryConsolidator {
	return &MemoryConsolidator{
		db:         db,
		qdrant:     qdrant,
		summarizer: summarizer,
	}
}

// Run starts the background consolidation cycle.
func (c *MemoryConsolidator) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Consolidate(ctx); err != nil {
				log.Printf("Consolidation error: %v", err)
			}
		}
	}
}

// Consolidate finds clusters of episodic memories and merges them into semantic ones.
func (c *MemoryConsolidator) Consolidate(ctx context.Context) error {
	// 1. Fetch episodic memories that haven't been reflected yet
	// For simplicity, we fetch memories older than 24h with low reinforcement
	memories, err := c.db.GetEpisodicCandidates(ctx, 24*time.Hour, 0.0)
	if err != nil {
		return err
	}

	if len(memories) < 3 {
		return nil // Not enough for clustering
	}

	// 2. Simple Semantic Clustering
	// We'll group by common keywords or entities for now, 
	// but a real implementation would use vector distance thresholds.
	clusters := c.simpleCluster(memories)

	for _, cluster := range clusters {
		if len(cluster) < 3 {
			continue
		}

		// 3. Summarize
		var contents []string
		for _, m := range cluster {
			contents = append(contents, m.Content)
		}

		summary, err := c.summarizer.Summarize(ctx, contents)
		if err != nil {
			log.Printf("Failed to summarize cluster: %v", err)
			continue
		}

		// 4. Create Semantic Memory
		semanticMemory := &storage.MemoryModel{
			TenantID:   cluster[0].TenantID,
			AgentID:    cluster[0].AgentID,
			Type:       "MEMORY_TYPE_SEMANTIC",
			Content:    summary,
			Importance: 0.8, // Consolidated knowledge is generally higher importance
			Metadata:   []byte(`{"consolidated": true}`),
		}

		if err := c.db.SaveMemory(ctx, semanticMemory); err != nil {
			log.Printf("Failed to save consolidated memory: %v", err)
			continue
		}

		// 5. Cleanup episodic fragments
		for _, m := range cluster {
			_ = c.db.MarkAsReflected(ctx, fmt.Sprintf("%x", m.ID.Bytes))
		}
		
		log.Printf("Consolidated %d memories into new semantic memory %x", len(cluster), semanticMemory.ID.Bytes)
	}

	return nil
}

func (c *MemoryConsolidator) simpleCluster(memories []*storage.MemoryModel) [][]*storage.MemoryModel {
	// Placeholder: In a real system, we would fetch embeddings from Qdrant 
	// and run a clustering algorithm (e.g. DBSCAN).
	// For now, we group everything into one big cluster if they share a tenant.
	groups := make(map[string][]*storage.MemoryModel)
	for _, m := range memories {
		tenantID := fmt.Sprintf("%x", m.TenantID.Bytes)
		groups[tenantID] = append(groups[tenantID], m)
	}

	var clusters [][]*storage.MemoryModel
	for _, g := range groups {
		clusters = append(clusters, g)
	}
	return clusters
}

// MockSummarizer for testing
type MockSummarizer struct{}

func (s *MockSummarizer) Summarize(ctx context.Context, memories []string) (string, error) {
	return fmt.Sprintf("Summary of %d interactions: %s...", len(memories), memories[0][:20]), nil
}
