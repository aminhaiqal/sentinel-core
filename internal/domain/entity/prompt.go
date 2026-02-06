package entity

import "time"

type AIRequest struct {
	UserID    string
	Prompt    string
	Provider  string
	Timestamp time.Time
}

type AIResponse struct {
	Content    string
	Cached     bool
	TokenCount int
	Cost       float64
}
