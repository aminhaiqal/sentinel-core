package usecase

import (
	"context"
	"fmt"
	"math/rand"
	"sentinel-core/internal/domain/entity"
	"sentinel-core/internal/domain/repository"
	"strings"
	"time"
)

type ResilientProvider struct {
	primary    repository.AIProvider
	fallback   repository.AIProvider // The "Plan B" (e.g., Gemini Flash)
	maxRetries int
	baseDelay  time.Duration
	timeout    time.Duration // The Safety Layer Timeout
}

func NewResilientProvider(primary, fallback repository.AIProvider) *ResilientProvider {
	return &ResilientProvider{
		primary:    primary,
		fallback:   fallback,
		maxRetries: 2, // Total 3 attempts for Primary
		baseDelay:  500 * time.Millisecond,
		timeout:    25 * time.Second, // Global cap per generation
	}
}

func (r *ResilientProvider) Generate(ctx context.Context, prompt string) (*entity.AIResponse, error) {
	// 1. Apply Timeout Layer
	// We create a scoped context so one slow request doesn't hang the whole server
	resCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// 2. Try Primary with Retries
	resp, err := r.executeWithRetry(resCtx, r.primary, prompt, "PRIMARY")
	if err == nil {
		return resp, nil
	}

	fmt.Printf("[RELIABILITY] Primary exhausted. Switching to FALLBACK. Error: %v\n", err)

	// 3. Tiered Fallback Flow
	// If primary fails, we try the fallback model ONCE (usually a faster/cheaper model)
	resp, err = r.fallback.Generate(resCtx, prompt)
	if err != nil {
		return nil, fmt.Errorf("both primary and fallback failed: %w", err)
	}

	// 4. Content Safety Note
	// Ensure metadata reflects that this came from the fallback
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]any)
	}

	resp.Metadata["fallback_used"] = true
	resp.Metadata["retry_count"] = 0

	return resp, nil
}

func (r *ResilientProvider) executeWithRetry(ctx context.Context, p repository.AIProvider, prompt, label string) (*entity.AIResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		resp, err := p.Generate(ctx, prompt)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if !r.isRetryable(err) || attempt == r.maxRetries {
			break
		}

		wait := r.calculateBackoff(attempt)
		select {
		case <-time.After(wait):
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (r *ResilientProvider) isRetryable(err error) bool {
	msg := strings.ToLower(err.Error())
	// Retry on Rate Limits (429) and Server Errors (5xx)
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "overloaded") ||
		strings.Contains(msg, "deadline")
}

func (r *ResilientProvider) calculateBackoff(attempt int) time.Duration {
	backoff := float64(r.baseDelay) * float64(int(1)<<attempt)
	jitter := (rand.Float64() * 0.2) * backoff // 20% jitter
	return time.Duration(backoff + jitter)
}
