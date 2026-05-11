package core

import (
	"math"
	"time"
)

// RankingWeights holds the coefficients for the ranking formula
type RankingWeights struct {
	Alpha float32 // Semantic similarity weight
	Beta  float32 // Temporal relevance weight
	Gamma float32 // Importance weight
	Delta float32 // Recency weight
}

// DefaultRankingWeights provides sensible defaults
var DefaultRankingWeights = RankingWeights{
	Alpha: 0.4, // Semantic similarity
	Beta:  0.2, // Temporal decay
	Gamma: 0.3, // User-marked importance
	Delta: 0.1, // Recency bonus
}

// MemoryScore represents a scored memory with breakdown
type MemoryScore struct {
	ID             string
	FinalScore     float32
	SemanticScore  float32
	TemporalScore  float32
	ImportanceScore float32
	RecencyScore   float32
}

// ComputeRank calculates R = α*S + β*T + γ*I + δ*C
// where:
//   S = semantic similarity (0-1)
//   T = temporal decay (0-1, decays exponentially)
//   I = importance (0-1, user-provided)
//   C = recency bonus (0-1, higher for recent)
func ComputeRank(
	semanticScore float32,
	importance float64,
	createdAt time.Time,
	weights RankingWeights,
) MemoryScore {
	// Clamp semantic score to [0, 1]
	s := float32(math.Min(1.0, math.Max(0.0, float64(semanticScore))))

	// Temporal decay: exponential decay over 30 days
	// After 30 days: score ~= 0.5
	// After 90 days: score ~= 0.125
	ageHours := float64(time.Since(createdAt).Hours())
	const decayHalfLife = 30 * 24.0 // 30 days in hours
	t := float32(math.Pow(0.5, ageHours/decayHalfLife))

	// Importance score (clamped to [0, 1])
	i := float32(math.Min(1.0, math.Max(0.0, importance)))

	// Recency bonus: extra boost for memories created in last 24 hours
	var c float32
	if ageHours < 24 {
		c = 1.0 - float32(ageHours/24.0)*0.5 // Decays from 1.0 to 0.5 over 24h
	} else {
		c = 0.0
	}

	// Normalize weights to sum to 1.0
	totalWeight := weights.Alpha + weights.Beta + weights.Gamma + weights.Delta
	alpha := weights.Alpha / totalWeight
	beta := weights.Beta / totalWeight
	gamma := weights.Gamma / totalWeight
	delta := weights.Delta / totalWeight

	// Compute final score
	finalScore := alpha*s + beta*t + gamma*i + delta*c

	return MemoryScore{
		FinalScore:      finalScore,
		SemanticScore:   s,
		TemporalScore:   t,
		ImportanceScore: i,
		RecencyScore:    c,
	}
}

// TemporalDecay calculates memory decay factor based on age
func TemporalDecay(createdAt time.Time) float32 {
	ageHours := float64(time.Since(createdAt).Hours())
	const decayHalfLife = 30 * 24.0 // 30 days in hours
	return float32(math.Pow(0.5, ageHours/decayHalfLife))
}

// ShouldConvertToSemantic determines if an episodic memory should be
// converted to semantic based on age and importance
func ShouldConvertToSemantic(createdAt time.Time, importance float64) bool {
	// Convert episodic to semantic if:
	// - Older than 7 days AND
	// - Importance score > 0.6 (significant memories)
	ageHours := float64(time.Since(createdAt).Hours())
	const conversionThresholdHours = 7 * 24.0 // 7 days

	return ageHours > conversionThresholdHours && importance > 0.6
}
