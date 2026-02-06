package usecase

import (
	"context"
	"sentinel-core/internal/domain/entity"
	"sentinel-core/internal/domain/repository"
)

type Orchestrator struct {
	vectorStore  repository.VectorStore
	tokenLimiter repository.TokenLimiter
	aiProvider   repository.AIProvider
	embedder     repository.Embedder
}

func NewOrchestrator(vs repository.VectorStore, tl repository.TokenLimiter, ai repository.AIProvider, emb repository.Embedder) *Orchestrator {
	return &Orchestrator{vectorStore: vs, tokenLimiter: tl, aiProvider: ai, embedder: emb}
}

func (u *Orchestrator) Execute(ctx context.Context, req entity.AIRequest) (*entity.AIResponse, error) {
	// 1. Check Rate Limits (Redis)
	allowed, _ := u.tokenLimiter.CheckLimit(ctx, req.UserID)
	if !allowed {
		return nil, entity.ErrRateLimitExceeded
	}

	// 2. Semantic Cache Lookup (Qdrant)
	cachedResp, err := u.vectorStore.Search(ctx, nil) // Simplify: embedding logic goes here
	if err == nil && cachedResp != nil {
		cachedResp.Cached = true
		return cachedResp, nil
	}

	// 3. Call AI Provider (Gemini/Claude)
	resp, err := u.aiProvider.Generate(ctx, req.Prompt)
	if err != nil {
		return nil, err
	}

	vector, _ := u.embedder.CreateEmbedding(ctx, req.Prompt)

	// 4. Background: Update usage and cache (Async)
	go u.vectorStore.Save(ctx, req.Prompt, resp, vector)
	go u.tokenLimiter.Increment(ctx, req.UserID, resp.TokenCount)

	return resp, nil
}
