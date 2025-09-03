package factory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hexarag/internal/adapters/llm/ollama"
	"hexarag/internal/adapters/llm/openai"
	"hexarag/internal/adapters/messaging/nats"
	"hexarag/internal/adapters/storage/sqlite"
	"hexarag/internal/adapters/tools/mcp"
	"hexarag/internal/domain/ports"
	"hexarag/internal/domain/services"
	"hexarag/internal/pkg/logutil"
	"hexarag/pkg/config"
)

// ServiceContainer holds all initialized services
type ServiceContainer struct {
	Storage                 ports.StoragePort
	Messaging               ports.MessagingPort
	LLM                     ports.LLMPort
	Tools                   ports.ToolPort
	ContextConstructor      *services.ContextConstructor
	InferenceEngine         *services.InferenceEngine
	MessageFlowOrchestrator *services.MessageFlowOrchestrator
	ModelManager            *services.ModelManager
	Logger                  *logutil.Logger
}

// InitializationOptions holds options for service initialization
type InitializationOptions struct {
	Config                config.Config
	ValidateConfiguration bool
	EnableHealthChecks    bool
	StartServices         bool
	Logger                *logutil.Logger
}

// ServiceFactory provides methods for creating and initializing services
type ServiceFactory struct {
	logger *logutil.Logger
}

// NewServiceFactory creates a new service factory
func NewServiceFactory(logger *logutil.Logger) *ServiceFactory {
	if logger == nil {
		logger = logutil.NewDefaultLogger()
	}

	return &ServiceFactory{
		logger: logger,
	}
}

// Initialize creates and initializes all services based on configuration
func (sf *ServiceFactory) Initialize(ctx context.Context, opts InitializationOptions) (*ServiceContainer, error) {
	if opts.Logger != nil {
		sf.logger = opts.Logger
	}

	sf.logger.Info("Starting service initialization", logutil.Fields{
		"validate_config":      opts.ValidateConfiguration,
		"enable_health_checks": opts.EnableHealthChecks,
		"start_services":       opts.StartServices,
	})

	// Validate configuration if requested
	if opts.ValidateConfiguration {
		// Note: For now using basic validation, enhanced factory would implement full validation
		if opts.Config.Server.Host == "" || opts.Config.Server.Port <= 0 {
			return nil, fmt.Errorf("invalid server configuration")
		}
		sf.logger.Info("Configuration validation passed")
	}

	container := &ServiceContainer{
		Logger: sf.logger,
	}

	// Initialize adapters (ports implementations)
	if err := sf.initializeAdapters(ctx, &opts.Config, container); err != nil {
		return nil, fmt.Errorf("failed to initialize adapters: %w", err)
	}

	// Initialize domain services
	if err := sf.initializeDomainServices(ctx, &opts.Config, container); err != nil {
		return nil, fmt.Errorf("failed to initialize domain services: %w", err)
	}

	// Perform health checks if enabled
	if opts.EnableHealthChecks {
		if err := sf.performHealthChecks(ctx, container); err != nil {
			return nil, fmt.Errorf("health checks failed: %w", err)
		}
		sf.logger.Info("All health checks passed")
	}

	// Start services if requested
	if opts.StartServices {
		if err := sf.startServices(ctx, container); err != nil {
			return nil, fmt.Errorf("failed to start services: %w", err)
		}
		sf.logger.Info("All services started successfully")
	}

	sf.logger.Info("Service initialization completed successfully")
	return container, nil
}

// initializeAdapters creates and configures all adapter instances
func (sf *ServiceFactory) initializeAdapters(ctx context.Context, cfg *config.Config, container *ServiceContainer) error {
	// Initialize storage adapter
	if err := sf.initializeStorageAdapter(ctx, cfg, container); err != nil {
		return fmt.Errorf("failed to initialize storage adapter: %w", err)
	}

	// Initialize messaging adapter
	if err := sf.initializeMessagingAdapter(ctx, cfg, container); err != nil {
		return fmt.Errorf("failed to initialize messaging adapter: %w", err)
	}

	// Initialize LLM adapter
	if err := sf.initializeLLMAdapter(ctx, cfg, container); err != nil {
		return fmt.Errorf("failed to initialize LLM adapter: %w", err)
	}

	// Initialize tools adapter
	if err := sf.initializeToolsAdapter(ctx, cfg, container); err != nil {
		return fmt.Errorf("failed to initialize tools adapter: %w", err)
	}

	return nil
}

// initializeStorageAdapter initializes the storage adapter based on configuration
func (sf *ServiceFactory) initializeStorageAdapter(ctx context.Context, cfg *config.Config, container *ServiceContainer) error {
	// This would be enhanced to support multiple storage backends
	// For now, we focus on the pattern
	sf.logger.Info("Initializing storage adapter", logutil.Fields{
		"type": "sqlite",
		"path": cfg.Database.Path,
	})

	// Storage adapter initialization would go here
	// container.Storage = storage adapter instance

	return nil
}

// initializeMessagingAdapter initializes the messaging adapter
func (sf *ServiceFactory) initializeMessagingAdapter(ctx context.Context, cfg *config.Config, container *ServiceContainer) error {
	sf.logger.Info("Initializing messaging adapter", logutil.Fields{
		"type":      "nats",
		"url":       cfg.NATS.URL,
		"jetstream": cfg.NATS.JetStream.Enabled,
	})

	// Messaging adapter initialization would go here
	// container.Messaging = messaging adapter instance

	return nil
}

// initializeLLMAdapter initializes the LLM adapter
func (sf *ServiceFactory) initializeLLMAdapter(ctx context.Context, cfg *config.Config, container *ServiceContainer) error {
	sf.logger.Info("Initializing LLM adapter", logutil.Fields{
		"provider": cfg.LLM.Provider,
		"base_url": cfg.LLM.BaseURL,
		"model":    cfg.LLM.Model,
	})

	// LLM adapter initialization would go here
	// container.LLM = llm adapter instance

	return nil
}

// initializeToolsAdapter initializes the tools adapter
func (sf *ServiceFactory) initializeToolsAdapter(ctx context.Context, cfg *config.Config, container *ServiceContainer) error {
	sf.logger.Info("Initializing tools adapter", logutil.Fields{
		"mcp_time_server_enabled": cfg.Tools.MCPTimeServer.Enabled,
	})

	// Tools adapter initialization would go here
	// container.Tools = tools adapter instance

	return nil
}

// initializeDomainServices creates domain services with proper dependencies
func (sf *ServiceFactory) initializeDomainServices(ctx context.Context, cfg *config.Config, container *ServiceContainer) error {
	// Initialize model manager first (dependency for other services)
	if err := sf.initializeModelManager(cfg, container); err != nil {
		return fmt.Errorf("failed to initialize model manager: %w", err)
	}

	// Initialize context constructor
	if err := sf.initializeContextConstructor(cfg, container); err != nil {
		return fmt.Errorf("failed to initialize context constructor: %w", err)
	}

	// Initialize inference engine
	if err := sf.initializeInferenceEngine(cfg, container); err != nil {
		return fmt.Errorf("failed to initialize inference engine: %w", err)
	}

	// Initialize message flow orchestrator
	if err := sf.initializeMessageFlowOrchestrator(cfg, container); err != nil {
		return fmt.Errorf("failed to initialize message flow orchestrator: %w", err)
	}

	return nil
}

// initializeModelManager creates the model manager service
func (sf *ServiceFactory) initializeModelManager(cfg *config.Config, container *ServiceContainer) error {
	sf.logger.Info("Initializing model manager service")

	// Model manager initialization would go here
	// container.ModelManager = services.NewModelManager(...)

	return nil
}

// initializeContextConstructor creates the context constructor service
func (sf *ServiceFactory) initializeContextConstructor(cfg *config.Config, container *ServiceContainer) error {
	sf.logger.Info("Initializing context constructor service")

	// Context constructor initialization would go here
	// container.ContextConstructor = services.NewContextConstructor(...)

	return nil
}

// initializeInferenceEngine creates the inference engine service
func (sf *ServiceFactory) initializeInferenceEngine(cfg *config.Config, container *ServiceContainer) error {
	sf.logger.Info("Initializing inference engine service")

	// Inference engine initialization would go here
	// container.InferenceEngine = services.NewInferenceEngine(...)

	return nil
}

// initializeMessageFlowOrchestrator creates the message flow orchestrator service
func (sf *ServiceFactory) initializeMessageFlowOrchestrator(cfg *config.Config, container *ServiceContainer) error {
	sf.logger.Info("Initializing message flow orchestrator service")

	// Message flow orchestrator initialization would go here
	// container.MessageFlowOrchestrator = services.NewMessageFlowOrchestrator(...)

	return nil
}

// performHealthChecks verifies all services are functioning correctly
func (sf *ServiceFactory) performHealthChecks(ctx context.Context, container *ServiceContainer) error {
	sf.logger.Info("Performing health checks")

	healthCheckTimeout := 5 * time.Second
	healthCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	// Check storage health
	if container.Storage != nil {
		if err := container.Storage.Ping(healthCtx); err != nil {
			return fmt.Errorf("storage health check failed: %w", err)
		}
		sf.logger.Debug("Storage health check passed")
	}

	// Check messaging health
	if container.Messaging != nil {
		if err := container.Messaging.Ping(); err != nil {
			return fmt.Errorf("messaging health check failed: %w", err)
		}
		sf.logger.Debug("Messaging health check passed")
	}

	return nil
}

// startServices starts all services that require background operations
func (sf *ServiceFactory) startServices(ctx context.Context, container *ServiceContainer) error {
	sf.logger.Info("Starting services")

	// Start context constructor
	if container.ContextConstructor != nil {
		if err := container.ContextConstructor.StartListening(ctx); err != nil {
			return fmt.Errorf("failed to start context constructor: %w", err)
		}
		sf.logger.Debug("Context constructor started")
	}

	// Start inference engine
	if container.InferenceEngine != nil {
		if err := container.InferenceEngine.StartListening(ctx); err != nil {
			return fmt.Errorf("failed to start inference engine: %w", err)
		}
		sf.logger.Debug("Inference engine started")
	}

	// Start message flow orchestrator
	if container.MessageFlowOrchestrator != nil {
		if err := container.MessageFlowOrchestrator.StartListening(ctx); err != nil {
			return fmt.Errorf("failed to start message flow orchestrator: %w", err)
		}
		sf.logger.Debug("Message flow orchestrator started")
	}

	return nil
}

// Shutdown gracefully shuts down all services
func (container *ServiceContainer) Shutdown(ctx context.Context) error {
	if container.Logger != nil {
		container.Logger.Info("Shutting down services")
	}

	// Close storage connection
	if container.Storage != nil {
		if closer, ok := container.Storage.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil && container.Logger != nil {
				container.Logger.Warn("Error closing storage", logutil.Fields{"error": err.Error()})
			}
		}
	}

	// Close messaging connection
	if container.Messaging != nil {
		if err := container.Messaging.Close(); err != nil && container.Logger != nil {
			container.Logger.Warn("Error closing messaging", logutil.Fields{"error": err.Error()})
		}
	}

	if container.Logger != nil {
		container.Logger.Info("Service shutdown completed")
	}

	return nil
}

// InitializeServices creates and initializes all services using a simplified approach
func InitializeServices(ctx context.Context, cfg *config.Config, logger *logutil.FieldLogger) (*ServiceContainer, error) {
	logger.Info("Starting service initialization")

	// Initialize storage adapter
	err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	storage, err := sqlite.NewAdapter(cfg.Database.Path, cfg.Database.MigrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Run database migrations
	if err := storage.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Initialize messaging adapter
	messaging, err := nats.NewAdapter(
		cfg.NATS.URL,
		cfg.NATS.JetStream.Enabled,
		cfg.NATS.JetStream.RetentionDays,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize messaging: %w", err)
	}

	// Initialize Ollama client and model manager
	ollamaBaseURL := strings.Replace(cfg.LLM.BaseURL, "/v1", "", 1)
	ollamaClient := ollama.NewClient(ollamaBaseURL)
	modelManager := services.NewModelManager(ollamaClient)

	// Validate default model exists (non-fatal)
	if err := modelManager.ValidateModel(ctx, cfg.LLM.Model); err != nil {
		logger.Warn("Default model not available", logutil.Fields{
			"model": cfg.LLM.Model,
			"error": err.Error(),
		})
		logger.Info("The system will continue but may not function properly until models are available")
	} else {
		logger.Info("Default model validated successfully", logutil.Fields{
			"model": cfg.LLM.Model,
		})
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
		return nil, fmt.Errorf("failed to initialize LLM adapter: %w", err)
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
		return nil, fmt.Errorf("failed to initialize context constructor: %w", err)
	}

	inferenceEngine := services.NewInferenceEngine(
		storage,
		messaging,
		llmAdapter,
		toolsAdapter,
	)

	// Initialize message flow orchestrator
	orchestratorConfig := &services.FlowOrchestratorConfig{
		DefaultModel:       cfg.LLM.Model,
		DefaultMaxTokens:   cfg.LLM.MaxTokens,
		DefaultTemperature: cfg.LLM.Temperature,
		EnableTools:        false, // Disable tools for models that don't support them
		Timeout:            30 * time.Second,
	}
	messageFlowOrchestrator := services.NewMessageFlowOrchestrator(storage, messaging, orchestratorConfig)

	// Create service container
	container := &ServiceContainer{
		Storage:                 storage,
		Messaging:               messaging,
		LLM:                     llmAdapter,
		Tools:                   toolsAdapter,
		ContextConstructor:      contextConstructor,
		InferenceEngine:         inferenceEngine,
		MessageFlowOrchestrator: messageFlowOrchestrator,
		ModelManager:            modelManager,
	}

	logger.Info("Service initialization completed successfully")
	return container, nil
}

// StartServices starts all services that require background operations
func (container *ServiceContainer) StartServices(ctx context.Context) error {
	// Start context constructor
	if err := container.ContextConstructor.StartListening(ctx); err != nil {
		return fmt.Errorf("failed to start context constructor: %w", err)
	}

	// Start inference engine
	if err := container.InferenceEngine.StartListening(ctx); err != nil {
		return fmt.Errorf("failed to start inference engine: %w", err)
	}

	// Start message flow orchestrator
	if err := container.MessageFlowOrchestrator.StartListening(ctx); err != nil {
		return fmt.Errorf("failed to start message flow orchestrator: %w", err)
	}

	return nil
}

// Close closes all service connections gracefully
func (container *ServiceContainer) Close() {
	// Close storage connection
	if container.Storage != nil {
		if closer, ok := container.Storage.(interface{ Close() error }); ok {
			closer.Close()
		}
	}

	// Close messaging connection
	if container.Messaging != nil {
		container.Messaging.Close()
	}
}
