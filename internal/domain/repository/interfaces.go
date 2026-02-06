package repository

import (
	"context"
	"sentinel-core/internal/domain/entity"
)

type VectorStore interface {
	Search(ctx context.Context, vector []float32, threshold float32) (*entity.AIResponse, error)
	Save(ctx context.Context, prompt string, resp *entity.AIResponse, vector []float32) error
}

type TokenLimiter interface {
	CheckLimit(ctx context.Context, userID string) (bool, error)
	Increment(ctx context.Context, userID string, tokens int) error
}

type AIProvider interface {
	Generate(ctx context.Context, prompt string) (*entity.AIResponse, error)
}

type Embedder interface {
	CreateEmbedding(ctx context.Context, text string) ([]float32, error)
}
