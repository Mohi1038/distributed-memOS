package core

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mohi1038/memos/internal/storage"
)

// ReflectionWorker handles background conversion of episodic memories to semantic
type ReflectionWorker struct {
	db          *storage.PostgresStore
	qdrant      *storage.QdrantStore
	embedder    EmbeddingGenerator
	summarizer  TextSummarizer
	batchSize   int
	interval    time.Duration
	stopChan    chan struct{}
}

// TextSummarizer creates semantic summaries from episodic memories
type TextSummarizer interface {
	Summarize(ctx context.Context, texts []string) (string, error)
}

// SimpleSummarizer does basic text summarization (MVP)
type SimpleSummarizer struct{}

func (s *SimpleSummarizer) Summarize(ctx context.Context, texts []string) (string, error) {
	if len(texts) == 0 {
		return "", fmt.Errorf("no texts to summarize")
	}

	// Simple MVP: concatenate with high-level abstraction
	if len(texts) == 1 {
		return fmt.Sprintf("Key learning: %s", texts[0]), nil
	}

	// Multi-memory summary
	summary := fmt.Sprintf("From %d related experiences: ", len(texts))
	patterns := extractPatterns(texts)
	summary += strings.Join(patterns, "; ")

	return summary, nil
}

// extractPatterns finds common themes in texts (simple MVP)
func extractPatterns(texts []string) []string {
	patterns := make(map[string]int)

	// Simple pattern extraction: look for common words
	for _, text := range texts {
		words := strings.Fields(strings.ToLower(text))
		for _, word := range words {
			if len(word) > 4 && !isCommonWord(word) {
				patterns[word]++
			}
		}
	}

	// Return top patterns
	var result []string
	for pattern, count := range patterns {
		if count > 1 {
			result = append(result, fmt.Sprintf("'%s' appears %d times", pattern, count))
		}
	}

	return result
}

func isCommonWord(word string) bool {
	common := map[string]bool{
		"the": true, "and": true, "that": true, "this": true,
		"with": true, "from": true, "have": true, "user": true,
		"time": true, "would": true, "there": true, "about": true,
	}
	return common[word]
}

// NewReflectionWorker creates a new reflection worker
func NewReflectionWorker(
	db *storage.PostgresStore,
	qdrant *storage.QdrantStore,
	embedder EmbeddingGenerator,
	summarizer TextSummarizer,
) *ReflectionWorker {
	return &ReflectionWorker{
		db:         db,
		qdrant:     qdrant,
		embedder:   embedder,
		summarizer: summarizer,
		batchSize:  5,
		interval:   12 * time.Hour, // Run every 12 hours
		stopChan:   make(chan struct{}),
	}
}

// Start begins the background reflection worker
func (w *ReflectionWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Println("Reflection worker started")

	// Run immediately on start
	w.reflect(ctx)

	// Then run periodically
	for {
		select {
		case <-ticker.C:
			w.reflect(ctx)
		case <-w.stopChan:
			log.Println("Reflection worker stopped")
			return
		case <-ctx.Done():
			log.Println("Reflection worker context cancelled")
			return
		}
	}
}

// Stop gracefully stops the worker
func (w *ReflectionWorker) Stop() {
	close(w.stopChan)
}

// reflect processes episodic memories for conversion to semantic
func (w *ReflectionWorker) reflect(ctx context.Context) {
	log.Println("Running reflection cycle...")

	// Get candidates: episodic memories older than 7 days with importance > 0.6
	candidates, err := w.db.GetEpisodicCandidates(ctx, 7*24*time.Hour, 0.6)
	if err != nil {
		log.Printf("Failed to fetch candidates: %v", err)
		return
	}

	if len(candidates) == 0 {
		log.Println("No candidates for reflection")
		return
	}

	log.Printf("Found %d candidates for reflection", len(candidates))

	// Group by agent and tenant for semantic clustering
	grouped := groupByAgentTenant(candidates)

	for key, memories := range grouped {
		w.processGroup(ctx, key, memories)
	}
}

// groupByAgentTenant groups memories by tenant+agent for semantic clustering
func groupByAgentTenant(memories []*storage.MemoryModel) map[string][]*storage.MemoryModel {
	groups := make(map[string][]*storage.MemoryModel)

	for _, mem := range memories {
		key := mem.TenantID.String() + ":" + mem.AgentID.String()
		groups[key] = append(groups[key], mem)
	}

	return groups
}

// processGroup converts a group of episodic memories to semantic
func (w *ReflectionWorker) processGroup(ctx context.Context, key string, memories []*storage.MemoryModel) {
	if len(memories) < 2 {
		return // Need at least 2 memories to extract patterns
	}

	// Extract content for summarization
	contents := make([]string, len(memories))
	for i, mem := range memories {
		contents[i] = mem.Content
	}

	// Create semantic summary
	summary, err := w.summarizer.Summarize(ctx, contents)
	if err != nil {
		log.Printf("Failed to summarize: %v", err)
		return
	}

	// Generate embedding for summary
	embedding, err := w.embedder.Generate(ctx, summary)
	if err != nil {
		log.Printf("Failed to embed summary: %v", err)
		return
	}

	// Create semantic memory
	semanticMem := &storage.MemoryModel{
		TenantID:   memories[0].TenantID,
		AgentID:    memories[0].AgentID,
		Type:       "MEMORY_TYPE_SEMANTIC",
		Content:    summary,
		Importance: 0.7, // Semantic memories have moderate importance
		Metadata:   memories[0].Metadata, // Keep original metadata
	}

	// Save to database
	if err := w.db.SaveMemory(ctx, semanticMem); err != nil {
		log.Printf("Failed to save semantic memory: %v", err)
		return
	}

	// Save embedding to Qdrant
	idStr := semanticMem.ID.String()
	payload := map[string]interface{}{
		"tenant_id": memories[0].TenantID.String(),
		"agent_id":  memories[0].AgentID.String(),
		"type":      "MEMORY_TYPE_SEMANTIC",
		"is_reflected": true,
	}
	if err := w.qdrant.UpsertMemory(ctx, "memories", idStr, embedding, payload); err != nil {
		log.Printf("Failed to save semantic memory to Qdrant: %v", err)
		// Not fatal - continue anyway
	}

	log.Printf("Created semantic memory: %s from %d episodic memories", idStr, len(memories))

	// Mark episodic memories as reflected
	for _, mem := range memories {
		if err := w.db.MarkAsReflected(ctx, mem.ID.String()); err != nil {
			log.Printf("Failed to mark memory as reflected: %v", err)
		}
	}
}
