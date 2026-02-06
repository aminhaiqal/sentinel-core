package client

import (
	"context"
	"sentinel-core/internal/domain/entity"

	"google.golang.org/genai"
)

type GeminiClient struct {
	client *genai.Client
	model  string
}

func NewGeminiClient(ctx context.Context, projectID, location, model string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, err
	}
	return &GeminiClient{client: client, model: model}, nil
}

func NewGeminiClientFromClient(c *genai.Client, model string) *GeminiClient {
	return &GeminiClient{
		client: c,
		model:  model,
	}
}

func (g *GeminiClient) Generate(ctx context.Context, prompt string) (*entity.AIResponse, error) {
	result, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(prompt), nil)
	if err != nil {
		return nil, err
	}

	// Assuming the result contains text and token info
	return &entity.AIResponse{
		Content:    result.Candidates[0].Content.Parts[0].Text, // simplified
		TokenCount: int(result.UsageMetadata.TotalTokenCount),
		Cached:     false,
	}, nil
}
