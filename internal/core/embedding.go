package core

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/sashabaranov/go-openai"
)

type EmbeddingGenerator interface {
	Generate(ctx context.Context, text string) ([]float32, error)
}

// MockEmbeddingGenerator for testing without API calls
type MockEmbeddingGenerator struct {
	Size int
}

func (g *MockEmbeddingGenerator) Generate(ctx context.Context, text string) ([]float32, error) {
	vec := make([]float32, g.Size)
	for i := range vec {
		vec[i] = rand.Float32()
	}
	return vec, nil
}

// OpenAIEmbeddingGenerator uses OpenAI's embedding API
type OpenAIEmbeddingGenerator struct {
	client    *openai.Client
	modelName string
}

func NewOpenAIEmbeddingGenerator(apiKey string, model string) *OpenAIEmbeddingGenerator {
	return &OpenAIEmbeddingGenerator{
		client:    openai.NewClient(apiKey),
		modelName: model,
	}
}

func (g *OpenAIEmbeddingGenerator) Generate(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	req := openai.EmbeddingRequest{
		Input:  []string{text},
		Model:  openai.EmbeddingModel(g.modelName),
	}

	resp, err := g.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %v", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned from API")
	}

	return resp.Data[0].Embedding, nil
}

// HybridEmbeddingGenerator tries OpenAI first, falls back to mock
type HybridEmbeddingGenerator struct {
	openAI *OpenAIEmbeddingGenerator
	mock   *MockEmbeddingGenerator
}

func NewHybridEmbeddingGenerator(apiKey string, model string, mockSize int) *HybridEmbeddingGenerator {
	return &HybridEmbeddingGenerator{
		openAI: NewOpenAIEmbeddingGenerator(apiKey, model),
		mock:   &MockEmbeddingGenerator{Size: mockSize},
	}
}

func (g *HybridEmbeddingGenerator) Generate(ctx context.Context, text string) ([]float32, error) {
	// Try OpenAI first
	embedding, err := g.openAI.Generate(ctx, text)
	if err == nil {
		return embedding, nil
	}

	// Fall back to mock if OpenAI fails
	fmt.Printf("OpenAI failed (%v), falling back to mock embeddings\n", err)
	return g.mock.Generate(ctx, text)
}
