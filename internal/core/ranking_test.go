// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
	"testing"
	"time"
)

func TestComputeRank(t *testing.T) {
	weights := DefaultRankingWeights
	now := time.Now()

	tests := []struct {
		name          string
		semanticScore float32
		importance    float64
		reinforcement float64
		decayFactor   float64
		createdAt     time.Time
		wantMin       float32
	}{
		{
			name:          "Recent high semantic",
			semanticScore: 0.9,
			importance:    0.8,
			reinforcement: 1.0,
			decayFactor:   0.1,
			createdAt:     now.Add(-1 * time.Hour),
			wantMin:       0.7,
		},
		{
			name:          "Old low semantic",
			semanticScore: 0.2,
			importance:    0.3,
			reinforcement: 0.0,
			decayFactor:   0.5, // Faster decay
			createdAt:     now.Add(-100 * 24 * time.Hour),
			wantMin:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ComputeRank(tt.semanticScore, tt.importance, tt.reinforcement, tt.decayFactor, tt.createdAt, weights)
			if score.FinalScore < tt.wantMin {
				t.Errorf("ComputeRank() final score = %v, want min %v", score.FinalScore, tt.wantMin)
			}
		})
	}
}

func BenchmarkComputeRank(b *testing.B) {
	weights := DefaultRankingWeights
	createdAt := time.Now().Add(-24 * time.Hour)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeRank(0.8, 0.7, 1.0, 0.1, createdAt, weights)
	}
}

func BenchmarkTemporalDecay(b *testing.B) {
	createdAt := time.Now().Add(-720 * time.Hour)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TemporalDecay(createdAt)
	}
}
