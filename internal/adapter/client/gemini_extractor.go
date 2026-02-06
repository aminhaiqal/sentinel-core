package client

import (
	"context"
	"encoding/json"
	"google.golang.org/genai"
)

type GeminiExtractor struct {
	client *genai.Client
	model  string
}

func NewGeminiExtractor(client *genai.Client, model string) *GeminiExtractor {
	return &GeminiExtractor{client: client, model: model}
}

func (e *GeminiExtractor) ExtractMetadata(ctx context.Context, prompt string) map[string]string {
	// We use a System Prompt to force JSON output
	instruction := `Extract key entities from the user prompt as a flat JSON object of strings. 
    Focus on 'action', 'source', and 'target'. 
    If not found, omit the key. Do not explain.
    Example: "Move money from Savings to Checking" -> {"action": "transfer", "source": "savings", "target": "checking"}`

	resp, err := e.client.Models.GenerateContent(ctx, e.model, genai.Text(instruction+"\nPrompt: "+prompt), nil)
	if err != nil {
		return nil
	}

	var metadata map[string]string
	// We attempt to unmarshal the AI's string response into our map
	err = json.Unmarshal([]byte(resp.Text()), &metadata)
	if err != nil {
		return nil
	}

	return metadata
}