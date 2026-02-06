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
}

func NewOrchestrator(vs repository.VectorStore, tl repository.TokenLimiter, ai repository.AIProvider, emb repository.Embedder, ev repository.Evaluator) *Orchestrator {
	return &Orchestrator{vectorStore: vs, tokenLimiter: tl, aiProvider: ai, embedder: emb, evaluator:    ev}
}

func (u *Orchestrator) Execute(ctx context.Context, req entity.AIRequest) (*entity.AIResponse, error) {
    // 1. Redaction (PII Masking) - Optional but recommended here
    // req.Prompt = u.redactor.Redact(req.Prompt)

    // 2. Check Rate Limits
    allowed, err := u.tokenLimiter.CheckLimit(ctx, req.UserID)
    if err != nil || !allowed {
        return nil, entity.ErrRateLimitExceeded
    }

    // 3. Generate Embedding
    vector, err := u.embedder.CreateEmbedding(ctx, req.Prompt)
    if err != nil {
        return nil, fmt.Errorf("embedding generation failed: %w", err)
    }

    // 4. Semantic Cache Lookup 
    // IMPORTANT: We use a slightly lower threshold here to "catch" candidates for the Judge
    const searchThreshold = 0.75 
    cachedResp, score, originalPrompt, err := u.vectorStore.Search(ctx, vector, searchThreshold)
    
    // LOG ALWAYS: This helps you debug when nothing happens
    if err != nil {
        fmt.Printf("[SENTINEL] Cache search error: %v\n", err)
    } else if cachedResp == nil {
        fmt.Printf("[SENTINEL] Cache Miss: No candidates found above %.2f\n", searchThreshold)
    }

    if cachedResp != nil {
        // TIER 1: Extreme Confidence
        if score > 0.96 {
            fmt.Printf("[SENTINEL] High-Confidence HIT (Score: %.4f)\n", score)
            cachedResp.Cached = true
            return cachedResp, nil
        }

        // TIER 2: The "Ambiguity Zone" - Ask the Judge
        fmt.Printf("[SENTINEL] Ambiguous Candidate (Score: %.4f). Original: \"%s\"\n", score, originalPrompt)
        fmt.Println("[SENTINEL] Calling Judge...")
        
        isMatch := u.evaluator.IsMatch(ctx, req.Prompt, originalPrompt)
        
        if isMatch {
            fmt.Println("[SENTINEL] Judge Approved: Intent matches. Returning Cache.")
            cachedResp.Cached = true
            return cachedResp, nil
        }
        
        fmt.Println("[SENTINEL] Judge Rejected: Intent differs. Proceeding to AI Provider.")
    }

    // 5. Call AI Provider
    resp, err := u.aiProvider.Generate(ctx, req.Prompt)
    if err != nil {
        return nil, err
    }

    // 6. Background Update
    go func() {
        bgCtx := context.Background()
        u.vectorStore.Save(bgCtx, req.Prompt, resp, vector)
        u.tokenLimiter.Increment(bgCtx, req.UserID, resp.TokenCount)
        fmt.Println("[SENTINEL] Background: Cache updated.")
    }()

    return resp, nil
}
