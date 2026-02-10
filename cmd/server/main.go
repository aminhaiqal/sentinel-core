package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"sentinel-core/internal/adapter/api"
	"sentinel-core/internal/adapter/client"
	"sentinel-core/internal/adapter/store"
	"sentinel-core/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
)

func main() {
	if err := godotenv.Load(".env.dev"); err != nil {
		log.Println("Warning: .env.dev file not found, using system environment variables")
	}
	ctx := context.Background()

	redisAddr := os.Getenv("REDIS_ADDR")
	qdrantHost := os.Getenv("QDRANT_HOST")
	qdrantPortStr := os.Getenv("QDRANT_PORT")
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	tokenLimitStr := os.Getenv("USER_TOKEN_LIMIT")

	qdrantPort, _ := strconv.Atoi(qdrantPortStr)
	tokenLimit, _ := strconv.Atoi(tokenLimitStr)

	// Redis for Rate Limiting
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Qdrant for Semantic Cache
	qClient, err := qdrant.NewClient(&qdrant.Config{
		Host: qdrantHost,
		Port: qdrantPort,
	})
	if err != nil {
		log.Fatalf("failed to connect to qdrant: %v", err)
	}

	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		log.Fatalf("failed to init genai client: %v", err)
	}

	primaryModel := client.NewGeminiClientFromClient(genaiClient, "gemini-2.5-flash")
	fallbackModel := client.NewGeminiClientFromClient(genaiClient, "gemini-1.5-flash")

	resilientProvider := usecase.NewResilientProvider(primaryModel, fallbackModel)

	embedder := client.NewEmbedderFromClient(genaiClient, "text-embedding-004")
	evaluator := client.NewGeminiEvaluator(genaiClient, "gemini-2.5-flash")
	extractor := client.NewGeminiExtractor(genaiClient, "gemini-3-flash")

	vectorStore := store.NewQdrantStore(qClient, os.Getenv("QDRANT_COLLECTION"))
	if err := vectorStore.InitCollection(ctx, 768); err != nil {
		log.Fatalf("failed to init qdrant collection: %v", err)
	}

	tokenLimiter := store.NewRedisLimiter(rdb, tokenLimit)

	// Inject the adapters into the Orchestration Layer
	orchestrator := usecase.NewOrchestrator(vectorStore, tokenLimiter, resilientProvider, embedder, evaluator, extractor)

	go func() {
		warmCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := embedder.CreateEmbedding(warmCtx, "warmup")
		if err != nil {
			log.Printf("[SENTINEL-WARMER] Embedder warm-up failed: %v", err)
		}

		// 2. Warm the LLM (Wakes up the model instance)
		_, err = resilientProvider.Generate(warmCtx, ".")
		if err != nil {
			log.Printf("[SENTINEL-WARMER] Gemini warm-up failed: %v", err)
		}

		log.Println("[SENTINEL-WARMER] Pre-warm complete. Gateway is HOT.")
	}()

	// Initialize API Layer (Delivery Layer)
	app := fiber.New(fiber.Config{
		AppName: "Sentinel-AI Gateway",
	})

	handler := api.NewPromptHandler(orchestrator)
	api.SetupRouter(app, handler)

	// Start Server
	log.Printf("Sentinel-AI Gateway running on port %s", os.Getenv("PORT"))
	log.Fatal(app.Listen(":" + os.Getenv("PORT")))
}
