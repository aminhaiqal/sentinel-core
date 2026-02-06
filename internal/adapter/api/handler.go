package api

import (
	"errors"
	"sentinel-core/internal/domain/entity"
	"sentinel-core/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

type PromptHandler struct {
	orchestrator *usecase.Orchestrator
}

func NewPromptHandler(orch *usecase.Orchestrator) *PromptHandler {
	return &PromptHandler{orchestrator: orch}
}

func (h *PromptHandler) HandlePrompt(c *fiber.Ctx) error {
	var req entity.AIRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	// The Delivery layer maps the business error to HTTP status codes
	resp, err := h.orchestrator.Execute(c.Context(), req)
	if err != nil {
		if errors.Is(err, entity.ErrRateLimitExceeded) {
			return c.Status(429).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(fiber.Map{"error": "internal gateway error"})
	}

	// Return response with custom headers to show off the "Sentinel" features
	c.Set("X-Sentinel-Cache-Hit", "false")
	if resp.Cached {
		c.Set("X-Sentinel-Cache-Hit", "true")
	}

	return c.Status(200).JSON(resp)
}
