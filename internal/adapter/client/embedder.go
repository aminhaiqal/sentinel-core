package client

import (
	"context"

	"google.golang.org/genai"
)

type Embedder struct {
	client *genai.Client
	model  string // e.g., "text-embedding-004"
}

func NewEmbedder(ctx context.Context, projectID, location, model string) (*Embedder, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, err
	}
	return &Embedder{
		client: client,
		model:  model,
	}, nil
}

func NewEmbedderFromClient(c *genai.Client, model string) *Embedder {
	return &Embedder{
		client: c,
		model:  model,
	}
}

func (e *Embedder) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	res, err := e.client.Models.EmbedContent(ctx, e.model, genai.Text(text), nil)
	if err != nil {
		return nil, err
	}
	return res.Embeddings[0].Values, nil
}
