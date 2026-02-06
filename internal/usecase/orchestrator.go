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
	evaluator    repository.Evaluator
	extractor 	 repository.Extractor
}

func NewOrchestrator(vs repository.VectorStore, tl repository.TokenLimiter, ai repository.AIProvider, emb repository.Embedder, ev repository.Evaluator, ex repository.Extractor) *Orchestrator {
	return &Orchestrator{vectorStore: vs, tokenLimiter: tl, aiProvider: ai, embedder: emb, evaluator: ev, extractor: ex}
}

func (u *Orchestrator) Execute(ctx context.Context, req entity.AIRequest) (*entity.AIResponse, error) {
    // 1. Check Rate Limits
    allowed, err := u.tokenLimiter.CheckLimit(ctx, req.UserID)
    if err != nil || !allowed {
        return nil, entity.ErrRateLimitExceeded
    }
	extractedMeta := u.extractor.ExtractMetadata(ctx, req.Prompt)
	fmt.Printf("[SENTINEL] Extracted Metadata: %v\n", extractedMeta)
    
	// 2. Generate Embedding
    vector, err := u.embedder.CreateEmbedding(ctx, req.Prompt)
    if err != nil {
        return nil, fmt.Errorf("embedding generation failed: %w", err)
    }

    // 3. Prepare Filters for Metadata (Fixes Test 3)
    // We add user_id by default to prevent cross-user data leaks
    filters := map[string]string{"user_id": req.UserID}
    for k, v := range extractedMeta {
        filters[k] = v
    }

    // 4. Semantic Cache Lookup with Filters
    cachedResp, score, originalPrompt, err := u.vectorStore.Search(ctx, vector, 0.75, extractedMeta)
    
    if err != nil {
        fmt.Printf("[SENTINEL] Cache search error: %v\n", err)
    }

    if cachedResp != nil {
        // TIER 1: Extreme Confidence
        if score > 0.98 { // Slightly increased for metadata-filtered hits
            fmt.Printf("[SENTINEL] High-Confidence HIT (Score: %.4f)\n", score)
            cachedResp.Cached = true
            return cachedResp, nil
        }

        // TIER 2: Ambiguity Zone
        fmt.Printf("[SENTINEL] Ambiguous Candidate (Score: %.4f). Calling Judge...\n", score)
        if u.evaluator.IsMatch(ctx, req.Prompt, originalPrompt) {
            fmt.Println("[SENTINEL] Judge Approved: Intent matches.")
            cachedResp.Cached = true
            return cachedResp, nil
        }
        fmt.Println("[SENTINEL] Judge Rejected: Intent differs.")
    }

    // 5. Call AI Provider
    resp, err := u.aiProvider.Generate(ctx, req.Prompt)
    if err != nil {
        return nil, err
    }

    // 6. Background Update (Including Metadata)
    go func() {
        bgCtx := context.Background()
        
        // Prepare the payload metadata to be saved
        saveMetadata := map[string]any{
            "user_id": req.UserID,
        }
        for k, v := range req.Metadata {
            saveMetadata[k] = v
        }

        u.vectorStore.Save(bgCtx, req.Prompt, resp, vector, saveMetadata)
        u.tokenLimiter.Increment(bgCtx, req.UserID, resp.TokenCount)
        fmt.Println("[SENTINEL] Background: Cache updated with metadata.")
    }()

    return resp, nil
}