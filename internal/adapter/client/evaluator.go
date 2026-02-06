package client

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type GeminiEvaluator struct {
	client *genai.Client
	model  string
}

func NewGeminiEvaluator(client *genai.Client, model string) *GeminiEvaluator {
	return &GeminiEvaluator{client: client, model: model}
}

func (e *GeminiEvaluator) IsMatch(ctx context.Context, userPrompt, cachedPrompt string) bool {
	// A highly structured prompt for deterministic YES/NO output
	instruction := `You are a Semantic Intent Judge. 
    Compare the following two user queries. 
    Are they asking for the same information, even if phrased differently?
    - If they have the same intent, respond ONLY with "YES".
    - If there is a nuance difference or they ask for different things, respond ONLY with "NO".`

	prompt := fmt.Sprintf("%s\n\nQuery 1: %s\nQuery 2: %s", instruction, userPrompt, cachedPrompt)

	resp, err := e.client.Models.GenerateContent(ctx, e.model, genai.Text(prompt), nil)
	if err != nil {
		return false // Default to safe 'No Match' on error
	}

	result := strings.TrimSpace(strings.ToUpper(resp.Text()))
	return strings.Contains(result, "YES")
}