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
	extractor    repository.Extractor
}

func NewOrchestrator(vs repository.VectorStore, tl repository.TokenLimiter, ai repository.AIProvider, emb repository.Embedder, ev repository.Evaluator, ex repository.Extractor) *Orchestrator {
	return &Orchestrator{vectorStore: vs, tokenLimiter: tl, aiProvider: ai, embedder: emb, evaluator: ev, extractor: ex}
}

func (u *Orchestrator) Execute(ctx context.Context, req entity.AIRequest) (*entity.AIResponse, error) {
	// 1. Guard Rail: Rate Limiting
	if err := u.validateRateLimit(ctx, req.UserID); err != nil {
		return nil, err
	}

	// 2. Pre-processing: Metadata & Embeddings
	extractedMeta := u.extractor.ExtractMetadata(ctx, req.Prompt)
	vector, err := u.embedder.CreateEmbedding(ctx, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	// 3. Cache Strategy: Try to find an existing answer
	if cachedResp := u.tryGetCachedResponse(ctx, req.Prompt, req.UserID, vector, extractedMeta); cachedResp != nil {
		return cachedResp, nil
	}

	// 4. Provider Strategy: Generate new answer
	resp, err := u.aiProvider.Generate(ctx, req.Prompt)
	if err != nil {
		return nil, err
	}

	// 5. Post-processing: Async updates
	u.asyncBackgroundUpdate(req, resp, vector, extractedMeta)

	return resp, nil
}

// --- Private Helpers ---

func (u *Orchestrator) validateRateLimit(ctx context.Context, userID string) error {
	allowed, err := u.tokenLimiter.CheckLimit(ctx, userID)
	if err != nil || !allowed {
		return entity.ErrRateLimitExceeded
	}
	return nil
}

func (u *Orchestrator) tryGetCachedResponse(ctx context.Context, prompt, userID string, vector []float32, meta map[string]string) *entity.AIResponse {
	// Prepare scoped filters (User ID + Extracted Intent)
	filters := map[string]string{"user_id": userID}
	for k, v := range meta {
		filters[k] = v
	}

	cachedResp, score, originalPrompt, err := u.vectorStore.Search(ctx, vector, 0.75, filters)
	if err != nil || cachedResp == nil {
		return nil
	}

	// TIER 1: Instant Hit
	if score > 0.98 {
		cachedResp.Cached = true
		return cachedResp
	}

	// TIER 2: Human-like evaluation (Judge)
	if u.evaluator.IsMatch(ctx, prompt, originalPrompt) {
		cachedResp.Cached = true
		return cachedResp
	}

	return nil
}

func (u *Orchestrator) asyncBackgroundUpdate(req entity.AIRequest, resp *entity.AIResponse, vector []float32, meta map[string]string) {
	go func() {
		bgCtx := context.Background()
		saveMeta := make(map[string]any)
		for k, v := range meta {
			saveMeta[k] = v
		}
		saveMeta["user_id"] = req.UserID

		_ = u.vectorStore.Save(bgCtx, req.Prompt, resp, vector, saveMeta)
		_ = u.tokenLimiter.Increment(bgCtx, req.UserID, resp.TokenCount)
	}()
}
