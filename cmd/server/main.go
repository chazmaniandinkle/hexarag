package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	httpapi "hexarag/internal/adapters/api/http"
	"hexarag/internal/adapters/websocket"
	"hexarag/internal/domain/metrics"
	"hexarag/internal/pkg/constants"
	"hexarag/internal/pkg/factory"
	"hexarag/internal/pkg/logutil"
	"hexarag/pkg/config"
)

func main() {
	// Initialize logger with proper configuration
	logger := logutil.NewDefaultLogger().WithFields(logutil.Fields{
		"component": "main",
		"service":   constants.ServiceName,
	})

	// Load and validate configuration
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal("Failed to load configuration", logutil.Fields{
			"error": err.Error(),
		})
	}

	// Use configuration validation utility
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Configuration validation failed", logutil.Fields{
			"errors": err.Error(),
		})
	}

	logger.Info("Starting HexaRAG server", logutil.Fields{
		"host":    cfg.Server.Host,
		"port":    cfg.Server.Port,
		"version": constants.ServiceVersion,
	})

	// Initialize services using factory pattern
	ctx := context.Background()
	serviceContainer, err := factory.InitializeServices(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize services", logutil.Fields{
			"error": err.Error(),
		})
	}
	defer serviceContainer.Close()

	// Start all services
	if err := serviceContainer.StartServices(ctx); err != nil {
		logger.Fatal("Failed to start services", logutil.Fields{
			"error": err.Error(),
		})
	}

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector()

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run(ctx) // Start hub in background

	// Initialize HTTP server with proper configuration
	if cfg.Logging.Level == constants.LogLevelDebug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Setup API handlers using service container
	apiHandlers := httpapi.NewAPIHandlers(
		serviceContainer.Storage,
		serviceContainer.Messaging,
		serviceContainer.ContextConstructor,
		serviceContainer.InferenceEngine,
		serviceContainer.MessageFlowOrchestrator,
		serviceContainer.ModelManager,
		metricsCollector,
		hub,
	)
	apiHandlers.SetupRoutes(router)

	// Setup WebSocket endpoint
	router.GET("/ws", hub.HandleWebSocket)

	// Create HTTP server with timeouts from constants
	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:        router,
		ReadTimeout:    constants.DefaultHTTPTimeout,
		WriteTimeout:   constants.DefaultHTTPTimeout,
		IdleTimeout:    constants.DefaultHTTPTimeout * 2,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Server starting", logutil.Fields{
			"address": server.Addr,
			"mode":    gin.Mode(),
		})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", logutil.Fields{
				"error": err.Error(),
			})
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Give outstanding requests time to complete using constant
	shutdownCtx, cancel := context.WithTimeout(context.Background(), constants.GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", logutil.Fields{
			"error": err.Error(),
		})
	}

	logger.Info("Server exited gracefully")
}
