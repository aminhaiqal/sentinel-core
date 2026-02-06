package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func SetupRouter(app *fiber.App, handler *PromptHandler) {
	// Middleware
	app.Use(logger.New())

	// API Versioning
	v1 := app.Group("/v1")

	// Endpoints
	v1.Post("/chat", handler.HandlePrompt)
}
