package entity

import "time"

type AIRequest struct {
	UserID   string `json:"user_id"`
	Prompt   string `json:"prompt"`
	Provider string `json:"provider"` // e.g., "gemini", "claude"
	Model    string `json:"model"`    // e.g., "gemini-2.5-flash"

	// Metadata is critical for Test 3.
	// Example: {"source": "Account A", "target": "Account B"}
	Metadata map[string]string `json:"metadata"`

	// Optional: Allow the user to tweak the "creativity" per request
	Temperature float32   `json:"temperature"`
	Timestamp   time.Time `json:"timestamp"`
}

type AIResponse struct {
	Content    string         `json:"content"`
	Cached     bool           `json:"cached"` // Was this from Qdrant?
	Score      float32        `json:"score"`  // Similarity score for debugging
	Model      string         `json:"model"`  // Which model actually answered?
	TokenCount int            `json:"token_count"`
	Cost       float64        `json:"cost"`
	Latency    int64          `json:"latency_ms"` // How fast was the response?
	Metadata   map[string]any `json:"metadata"`
}
