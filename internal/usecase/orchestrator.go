package usecase

import (
	"context"
	"fmt"
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
    fmt.Printf("[SENTINEL] Processing request for User: %s\n", req.UserID)

    // 1. Check Rate Limits
    allowed, err := u.tokenLimiter.CheckLimit(ctx, req.UserID)
    if err != nil {
        return nil, fmt.Errorf("rate limiter check failed: %w", err)
    }
    if !allowed {
        fmt.Printf("[SENTINEL] Rate limit EXCEEDED for User: %s\n", req.UserID)
        return nil, entity.ErrRateLimitExceeded
    }

    // 2. Generate Embedding
    fmt.Println("[SENTINEL] Generating embedding for prompt...")
    vector, err := u.embedder.CreateEmbedding(ctx, req.Prompt)
    if err != nil {
        return nil, fmt.Errorf("embedding generation failed: %w", err)
    }

    // 3. Semantic Cache Lookup
    fmt.Println("[SENTINEL] Searching semantic cache (Qdrant)...")
    cachedResp, err := u.vectorStore.Search(ctx, vector, 0.80)
    if err == nil && cachedResp != nil {
        fmt.Println("[SENTINEL] CACHE HIT! Returning saved response.")
        cachedResp.Cached = true
        return cachedResp, nil
    }
    fmt.Println("[SENTINEL] Cache miss. Forwarding to AI Provider.")

    // 4. Call AI Provider (Gemini/Claude)
    resp, err := u.aiProvider.Generate(ctx, req.Prompt)
    if err != nil {
        return nil, fmt.Errorf("AI provider generation failed: %w", err)
    }
    fmt.Printf("[SENTINEL] AI response received. Tokens: %d\n", resp.TokenCount)

    // 5. Background: Update usage and cache (Async)
    fmt.Println("[SENTINEL] Saving to cache and updating usage in background...")
    go func() {
        // We use context.Background() because the request context might expire
        bgCtx := context.Background()
        u.vectorStore.Save(bgCtx, req.Prompt, resp, vector)
        u.tokenLimiter.Increment(bgCtx, req.UserID, resp.TokenCount)
    }()

    return resp, nil
}