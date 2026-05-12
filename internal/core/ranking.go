// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
	"math"
	"time"
)

// RankingWeights holds the coefficients for the ranking formula
type RankingWeights struct {
	Alpha float32 // Semantic similarity weight
	Beta  float32 // Temporal decay (adaptive) weight
	Gamma float32 // Importance weight
	Delta float32 // Reinforcement weight
}

// DefaultRankingWeights provides sensible defaults
var DefaultRankingWeights = RankingWeights{
	Alpha: 0.4, // Semantic similarity
	Beta:  0.3, // Adaptive Temporal decay
	Gamma: 0.2, // User-marked importance
	Delta: 0.1, // Reinforcement bonus
}

// MemoryScore represents a scored memory with breakdown
type MemoryScore struct {
	ID                 string
	FinalScore         float32
	SemanticScore      float32
	TemporalScore      float32
	ImportanceScore     float32
	ReinforcementScore float32
}

// ComputeRank calculates R = α*S + β*T + γ*I + δ*R'
// where:
//   S = semantic similarity (0-1)
//   T = adaptive temporal decay (Retention(t) = Importance * e^{-λt})
//   I = importance (0-1, user-provided)
//   R' = reinforcement score (normalized 0-1)
func ComputeRank(
	semanticScore float32,
	importance float64,
	reinforcement float64,
	decayFactor float64,
	createdAt time.Time,
	weights RankingWeights,
) MemoryScore {
	// Clamp semantic score to [0, 1]
	s := float32(math.Min(1.0, math.Max(0.0, float64(semanticScore))))

	// Adaptive Temporal decay: R(t) = I * e^(-λ * t)
	// λ is the decay_factor, t is age in days
	ageDays := time.Since(createdAt).Hours() / 24.0
	// λ defaults to 0.1 (standard), higher λ = faster decay
	lambda := math.Max(0.01, decayFactor) // Clamp to avoid infinity
	t := float32(math.Min(1.0, importance*math.Exp(-lambda*ageDays)))

	// Importance score (clamped to [0, 1])
	i := float32(math.Min(1.0, math.Max(0.0, importance)))

	// Reinforcement score (normalized via hyperbolic tangent to [0, 1])
	rPrime := float32(math.Tanh(reinforcement / 10.0))

	// Normalize weights to sum to 1.0
	totalWeight := weights.Alpha + weights.Beta + weights.Gamma + weights.Delta
	alpha := weights.Alpha / totalWeight
	beta := weights.Beta / totalWeight
	gamma := weights.Gamma / totalWeight
	delta := weights.Delta / totalWeight

	// Compute final score
	finalScore := alpha*s + beta*t + gamma*i + delta*rPrime

	return MemoryScore{
		FinalScore:         finalScore,
		SemanticScore:      s,
		TemporalScore:      t,
		ImportanceScore:    i,
		ReinforcementScore: rPrime,
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
