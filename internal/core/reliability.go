package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker protects external calls from cascading failures
type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failureCount int
	threshold    int
	timeout      time.Duration
	lastFailure  time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     StateClosed,
	}
}

// Execute runs the function protected by the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	cb.mu.Lock()
	
	// Check state
	if cb.state == StateOpen {
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = StateHalfOpen
		} else {
			cb.mu.Unlock()
			return nil, errors.New("circuit breaker is open")
		}
	}
	cb.mu.Unlock()

	// Execute call
	result, err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()
		if cb.failureCount >= cb.threshold {
			cb.state = StateOpen
		}
		return nil, err
	}

	// Success
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failureCount = 0
	}
	
	return result, nil
}

// Wrapper for Generate embedding calls
type ReliableEmbedder struct {
	inner   EmbeddingGenerator
	breaker *CircuitBreaker
}

func NewReliableEmbedder(inner EmbeddingGenerator) *ReliableEmbedder {
	return &ReliableEmbedder{
		inner:   inner,
		breaker: NewCircuitBreaker(3, 30*time.Second),
	}
}

func (re *ReliableEmbedder) Generate(ctx context.Context, text string) ([]float32, error) {
	res, err := re.breaker.Execute(func() (interface{}, error) {
		return re.inner.Generate(ctx, text)
	})
	if err != nil {
		return nil, err
	}
	return res.([]float32), nil
}
