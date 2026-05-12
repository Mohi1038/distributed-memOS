// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/mohi1038/memos/internal/storage"
)

// ConflictType classifies the type of contradiction found
type ConflictType string

const (
	ConflictTypeFactual   ConflictType = "factual"    // Direct factual contradiction ("X is deprecated" vs "X is active")
	ConflictTypeTemporal  ConflictType = "temporal"   // Same fact changes over time
	ConflictTypeNone      ConflictType = "none"
)

// Conflict represents a detected contradiction between two memories
type Conflict struct {
	MemoryA      *storage.MemoryModel
	MemoryB      *storage.MemoryModel
	Type         ConflictType
	WinnerID     string // resolved winner
	Explanation  string
}

// ConflictResolver detects and resolves contradictions between memories
type ConflictResolver struct {
	db    *storage.PostgresStore
	graph *storage.Neo4jStore
}

func NewConflictResolver(db *storage.PostgresStore, graph *storage.Neo4jStore) *ConflictResolver {
	return &ConflictResolver{db: db, graph: graph}
}

// DetectAndResolve checks if a newly stored memory contradicts any existing ones.
// It queries the graph for related memories and runs contradiction heuristics.
func (cr *ConflictResolver) DetectAndResolve(ctx context.Context, newMemory *storage.MemoryModel) {
	if cr.graph == nil {
		return
	}

	newID := newMemory.ID.String()
	tenantID := newMemory.TenantID.String()

	// Find related memories via shared graph entities
	relatedIDs, err := cr.graph.GetRelatedMemoryIDsForTenant(ctx, tenantID, newID, 1, 20)
	if err != nil {
		log.Printf("[Conflict] Failed to query related memories: %v", err)
		return
	}

	if len(relatedIDs) == 0 {
		return
	}

	for _, relID := range relatedIDs {
		if relID == newID {
			continue
		}

		existing, err := cr.db.GetMemoryByIDRaw(ctx, relID)
		if err != nil {
			continue
		}

		conflict := cr.detectConflict(newMemory, existing)
		if conflict.Type == ConflictTypeNone {
			continue
		}

		log.Printf("[Conflict] Detected %s conflict between %s and %s", conflict.Type, newID, relID)
		cr.resolve(ctx, conflict)
	}
}

// detectConflict applies heuristic rules to find contradictions
func (cr *ConflictResolver) detectConflict(a, b *storage.MemoryModel) Conflict {
	aLow := strings.ToLower(a.Content)
	bLow := strings.ToLower(b.Content)

	// --- Rule 1: Negation pairs ---
	// "X is deprecated" vs "X is active/available/working"
	negationPairs := [][2][]string{
		{
			{"deprecated", "removed", "deleted", "discontinued", "shut down", "broken", "offline"},
			{"active", "available", "working", "live", "running", "online", "restored"},
		},
		{
			{"disabled", "off", "blocked"},
			{"enabled", "on", "allowed"},
		},
		{
			{"failed", "down", "unreachable"},
			{"recovered", "healthy", "up", "available"},
		},
	}

	for _, pair := range negationPairs {
		negatives, positives := pair[0], pair[1]
		aIsNeg := containsAny(aLow, negatives)
		bIsPos := containsAny(bLow, positives)
		aIsPos := containsAny(aLow, positives)
		bIsNeg := containsAny(bLow, negatives)

		if (aIsNeg && bIsPos) || (aIsPos && bIsNeg) {
			return Conflict{
				MemoryA:     a,
				MemoryB:     b,
				Type:        ConflictTypeFactual,
				Explanation: "Negation pair detected",
			}
		}
	}

	// --- Rule 2: Same-subject temporal conflict ---
	// Same entities mentioned, but content clearly changed over time (> 7 day gap)
	if time.Since(a.CreatedAt) > 7*24*time.Hour && time.Since(b.CreatedAt) < 24*time.Hour {
		// b is fresh, a is stale — potential temporal conflict
		sharedWords := sharedSignificantWords(aLow, bLow)
		if len(sharedWords) >= 3 {
			return Conflict{
				MemoryA:     a,
				MemoryB:     b,
				Type:        ConflictTypeTemporal,
				Explanation: "Temporal overlap on shared subject",
			}
		}
	}

	return Conflict{Type: ConflictTypeNone}
}

// resolve applies the resolution strategy and marks the loser as conflicted
func (cr *ConflictResolver) resolve(ctx context.Context, c Conflict) {
	var winner, loser *storage.MemoryModel

	switch c.Type {
	case ConflictTypeFactual:
		// Freshness wins for factual conflicts: newer memory is trusted
		if c.MemoryA.CreatedAt.After(c.MemoryB.CreatedAt) {
			winner, loser = c.MemoryA, c.MemoryB
		} else {
			winner, loser = c.MemoryB, c.MemoryA
		}

		// Importance tiebreaker
		if c.MemoryA.CreatedAt.Equal(c.MemoryB.CreatedAt) {
			if c.MemoryA.Importance > c.MemoryB.Importance {
				winner, loser = c.MemoryA, c.MemoryB
			} else {
				winner, loser = c.MemoryB, c.MemoryA
			}
		}

	case ConflictTypeTemporal:
		// Newer always wins for temporal
		if c.MemoryA.CreatedAt.After(c.MemoryB.CreatedAt) {
			winner, loser = c.MemoryA, c.MemoryB
		} else {
			winner, loser = c.MemoryB, c.MemoryA
		}
	}

	if winner == nil || loser == nil {
		return
	}

	log.Printf("[Conflict] Resolution: winner=%s loser=%s type=%s reason=%s",
		winner.ID.String(), loser.ID.String(), c.Type, c.Explanation)

	// Reduce the loser's importance (soft deprecation, not deletion)
	// This naturally pushes it down in cognitive ranking
	if err := cr.db.ReduceImportance(ctx, loser.ID.String(), 0.1); err != nil {
		log.Printf("[Conflict] Failed to reduce loser importance: %v", err)
	}

	// Mark the contradicts relationship in Neo4j
	if cr.graph != nil {
		if err := cr.graph.MarkConflict(ctx, winner.ID.String(), loser.ID.String(), string(c.Type)); err != nil {
			log.Printf("[Conflict] Failed to mark conflict in graph: %v", err)
		}
	}
}

// --- Helpers ---

func containsAny(text string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}

func sharedSignificantWords(a, b string) []string {
	stopWords := map[string]bool{
		"the": true, "and": true, "is": true, "in": true, "of": true,
		"to": true, "a": true, "for": true, "that": true, "this": true,
	}

	aWords := make(map[string]bool)
	for _, w := range strings.Fields(a) {
		if len(w) > 3 && !stopWords[w] {
			aWords[w] = true
		}
	}

	var shared []string
	for _, w := range strings.Fields(b) {
		if len(w) > 3 && !stopWords[w] && aWords[w] {
			shared = append(shared, w)
		}
	}
	return shared
}
