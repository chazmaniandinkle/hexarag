package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	httpapi "github.com/username/hexarag/internal/adapters/api/http"
	"github.com/username/hexarag/internal/adapters/api/websocket"
	"github.com/username/hexarag/internal/adapters/llm/ollama"
	"github.com/username/hexarag/internal/adapters/llm/openai"
	"github.com/username/hexarag/internal/adapters/messaging/nats"
	"github.com/username/hexarag/internal/adapters/storage/sqlite"
	"github.com/username/hexarag/internal/adapters/tools/mcp"
	"github.com/username/hexarag/internal/domain/services"
	"github.com/username/hexarag/pkg/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting HexaRAG server on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Initialize storage adapter
	err = os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755)
	if err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	storage, err := sqlite.NewAdapter(cfg.Database.Path, cfg.Database.MigrationsPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Run database migrations
	ctx := context.Background()
	if err := storage.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize messaging adapter
	messaging, err := nats.NewAdapter(
		cfg.NATS.URL,
		cfg.NATS.JetStream.Enabled,
		cfg.NATS.JetStream.RetentionDays,
	)
	if err != nil {
		log.Fatalf("Failed to initialize messaging: %v", err)
	}
	defer messaging.Close()

	// Initialize Ollama client and model manager
	ollamaBaseURL := strings.Replace(cfg.LLM.BaseURL, "/v1", "", 1)
	ollamaClient := ollama.NewClient(ollamaBaseURL)
	modelManager := services.NewModelManager(ollamaClient)

	// Validate default model exists (non-fatal)
	if err := modelManager.ValidateModel(ctx, cfg.LLM.Model); err != nil {
		log.Printf("Warning: Default model %s not available: %v", cfg.LLM.Model, err)
		log.Printf("The system will continue but may not function properly until models are available")
	} else {
		log.Printf("Default model %s validated successfully", cfg.LLM.Model)
	}

	// Initialize LLM adapter
	llmAdapter, err := openai.NewAdapter(
		cfg.LLM.BaseURL,
		cfg.LLM.APIKey,
		cfg.LLM.Model,
		cfg.LLM.Provider,
		modelManager,
	)
	if err != nil {
		log.Fatalf("Failed to initialize LLM adapter: %v", err)
	}

	// Initialize tools adapter
	toolsAdapter := mcp.NewTimeServerAdapter(
		cfg.Tools.MCPTimeServer.Enabled,
		cfg.Tools.MCPTimeServer.Timezones,
	)

	// Initialize core services
	contextConstructor, err := services.NewContextConstructor(
		storage,
		messaging,
		cfg.LLM.Model,
		cfg.LLM.MaxTokens,
	)
	if err != nil {
		log.Fatalf("Failed to initialize context constructor: %v", err)
	}

	inferenceEngine := services.NewInferenceEngine(
		storage,
		messaging,
		llmAdapter,
		toolsAdapter,
	)

	// Start services
	if err := contextConstructor.StartListening(ctx); err != nil {
		log.Fatalf("Failed to start context constructor: %v", err)
	}

	if err := inferenceEngine.StartListening(ctx); err != nil {
		log.Fatalf("Failed to start inference engine: %v", err)
	}

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(messaging)
	if err := wsHub.Start(ctx); err != nil {
		log.Fatalf("Failed to start WebSocket hub: %v", err)
	}

	// Initialize HTTP server
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Setup API handlers
	apiHandlers := httpapi.NewAPIHandlers(storage, messaging, contextConstructor, inferenceEngine, modelManager)
	apiHandlers.SetupRoutes(router)

	// Setup WebSocket endpoint
	router.GET("/ws", wsHub.HandleWebSocket)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on http://%s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
