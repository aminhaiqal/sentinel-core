package api

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func SetupRouter(app *fiber.App, handler *PromptHandler) {
	// Middleware
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "healthy",
			"version": os.Getenv("APP_VERSION"),
			"env":     os.Getenv("ENV"),
		})
	})

	// API Versioning
	v1 := app.Group("/v1")
	// Endpoints
	v1.Post("/chat", handler.HandlePrompt)
}
