package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"hexarag/internal/adapters/llm/ollama"
	"hexarag/internal/domain/ports"
)

// OllamaClientInterface defines the interface for Ollama client operations
type OllamaClientInterface interface {
	ListModels(ctx context.Context) ([]ollama.Model, error)
	ShowModel(ctx context.Context, name string) (*ollama.ModelInfo, error)
	PullModel(ctx context.Context, name string, progressFn func(ollama.PullProgress)) error
	DeleteModel(ctx context.Context, name string) error
	GetRunningModels(ctx context.Context) ([]ollama.RunningModel, error)
	Ping(ctx context.Context) error
}

// ModelManager handles model lifecycle and caching
type ModelManager struct {
	ollamaClient OllamaClientInterface
	cache        *ModelCache
	mu           sync.RWMutex
}

// ModelCache stores model information with TTL
type ModelCache struct {
	models    []ports.Model
	timestamp time.Time
	ttl       time.Duration
}

// NewModelManager creates a new model manager
func NewModelManager(ollamaClient OllamaClientInterface) *ModelManager {
	return &ModelManager{
		ollamaClient: ollamaClient,
		cache: &ModelCache{
			models:    nil,
			timestamp: time.Time{},
			ttl:       5 * time.Minute, // Cache models for 5 minutes
		},
	}
}

// GetAvailableModels returns cached or fresh model list
func (mm *ModelManager) GetAvailableModels(ctx context.Context) ([]ports.Model, error) {
	mm.mu.RLock()
	
	// Check if cache is valid
	if mm.cache.models != nil && time.Since(mm.cache.timestamp) < mm.cache.ttl {
		models := make([]ports.Model, len(mm.cache.models))
		copy(models, mm.cache.models)
		mm.mu.RUnlock()
		return models, nil
	}
	
	mm.mu.RUnlock()

	// Cache is stale or empty, refresh it
	return mm.RefreshCache(ctx)
}

// RefreshCache forces a cache refresh from Ollama
func (mm *ModelManager) RefreshCache(ctx context.Context) ([]ports.Model, error) {
	// Check if Ollama is available first
	if err := mm.ollamaClient.Ping(ctx); err != nil {
		log.Printf("Ollama not available, falling back to empty model list: %v", err)
		return mm.getFallbackModels(), nil
	}

	// Get models from Ollama
	ollamaModels, err := mm.ollamaClient.ListModels(ctx)
	if err != nil {
		log.Printf("Failed to get models from Ollama: %v", err)
		return mm.getFallbackModels(), nil
	}

	// Convert to ports.Model format
	models := make([]ports.Model, len(ollamaModels))
	for i, model := range ollamaModels {
		models[i] = mm.convertOllamaModel(model)
	}

	// Update cache
	mm.mu.Lock()
	mm.cache.models = models
	mm.cache.timestamp = time.Now()
	mm.mu.Unlock()

	log.Printf("Refreshed model cache with %d models", len(models))
	return models, nil
}

// ValidateModel checks if a model exists in Ollama
func (mm *ModelManager) ValidateModel(ctx context.Context, modelName string) error {
	models, err := mm.GetAvailableModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to get available models: %w", err)
	}

	for _, model := range models {
		if model.ID == modelName || model.Name == modelName {
			return nil
		}
	}

	return fmt.Errorf("model '%s' not found in available models", modelName)
}

// GetModelInfo returns detailed information about a specific model
func (mm *ModelManager) GetModelInfo(ctx context.Context, modelName string) (*ports.Model, error) {
	// First check if model exists in cache
	models, err := mm.GetAvailableModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get available models: %w", err)
	}

	var baseModel *ports.Model
	for _, model := range models {
		if model.ID == modelName || model.Name == modelName {
			baseModel = &model
			break
		}
	}

	if baseModel == nil {
		return nil, fmt.Errorf("model '%s' not found", modelName)
	}

	// Try to get additional details from Ollama
	if err := mm.ollamaClient.Ping(ctx); err == nil {
		if info, err := mm.ollamaClient.ShowModel(ctx, modelName); err == nil {
			// Enhance model with additional details
			enhanced := *baseModel
			if info.Details.Family != "" {
				enhanced.Family = info.Details.Family
			}
			if info.Details.ParameterSize != "" {
				enhanced.Parameters = info.Details.ParameterSize
			}
			return &enhanced, nil
		}
	}

	return baseModel, nil
}

// convertOllamaModel converts an Ollama model to our ports.Model format
func (mm *ModelManager) convertOllamaModel(ollamaModel ollama.Model) ports.Model {
	// Extract model family and parameters from name if possible
	family := mm.extractModelFamily(ollamaModel.Name)
	parameters := mm.extractParameters(ollamaModel.Name)
	
	// Use details if available
	if ollamaModel.Details != nil {
		if ollamaModel.Details.Family != "" {
			family = ollamaModel.Details.Family
		}
		if ollamaModel.Details.ParameterSize != "" {
			parameters = ollamaModel.Details.ParameterSize
		}
	}

	return ports.Model{
		ID:          ollamaModel.Name,
		Name:        mm.formatModelName(ollamaModel.Name),
		Description: mm.generateModelDescription(ollamaModel.Name, family, parameters),
		Size:        ollamaModel.Size,
		Family:      family,
		Parameters:  parameters,
		Available:   true,
		ModifiedAt:  ollamaModel.ModifiedAt,
	}
}

// extractModelFamily extracts the model family from the model name
func (mm *ModelManager) extractModelFamily(name string) string {
	name = strings.ToLower(name)
	
	if strings.Contains(name, "llama") {
		return "llama"
	}
	if strings.Contains(name, "mistral") {
		return "mistral"
	}
	if strings.Contains(name, "deepseek") {
		return "deepseek"
	}
	if strings.Contains(name, "qwen") {
		return "qwen"
	}
	if strings.Contains(name, "phi") {
		return "phi"
	}
	if strings.Contains(name, "gemma") {
		return "gemma"
	}
	
	return "unknown"
}

// extractParameters extracts parameter information from model name
func (mm *ModelManager) extractParameters(name string) string {
	name = strings.ToLower(name)
	
	// Common parameter patterns
	if strings.Contains(name, "70b") || strings.Contains(name, "72b") {
		return "70B"
	}
	if strings.Contains(name, "34b") {
		return "34B"
	}
	if strings.Contains(name, "13b") {
		return "13B"
	}
	if strings.Contains(name, "8b") {
		return "8B"
	}
	if strings.Contains(name, "7b") {
		return "7B"
	}
	if strings.Contains(name, "3b") {
		return "3B"
	}
	if strings.Contains(name, "1b") {
		return "1B"
	}
	
	return "Unknown"
}

// formatModelName creates a user-friendly display name
func (mm *ModelManager) formatModelName(name string) string {
	// Split on colon to separate name from tag
	parts := strings.Split(name, ":")
	if len(parts) > 1 {
		modelName := parts[0]
		tag := parts[1]
		
		// Capitalize first letter of each word
		words := strings.Split(modelName, "-")
		for i, word := range words {
			if len(word) > 0 {
				words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
		
		return strings.Join(words, " ") + " (" + tag + ")"
	}
	
	return name
}

// generateModelDescription creates a description based on available information
func (mm *ModelManager) generateModelDescription(name, family, parameters string) string {
	if family == "unknown" && parameters == "Unknown" {
		return fmt.Sprintf("Language model: %s", name)
	}
	
	var parts []string
	if family != "unknown" {
		parts = append(parts, strings.Title(family))
	}
	if parameters != "Unknown" {
		parts = append(parts, parameters+" parameters")
	}
	
	if len(parts) > 0 {
		return strings.Join(parts, " model with ")
	}
	
	return fmt.Sprintf("Language model: %s", name)
}

// getFallbackModels returns a basic model list when Ollama is unavailable
func (mm *ModelManager) getFallbackModels() []ports.Model {
	return []ports.Model{
		{
			ID:          "deepseek-r1:8b",
			Name:        "DeepSeek R1 8B",
			Description: "DeepSeek's reasoning model with 8B parameters (offline)",
			Available:   false,
			Family:      "deepseek",
			Parameters:  "8B",
		},
	}
}

// ClearCache clears the model cache, forcing next request to refresh
func (mm *ModelManager) ClearCache() {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	mm.cache.models = nil
	mm.cache.timestamp = time.Time{}
}

// IsModelAvailable checks if a specific model is currently available
func (mm *ModelManager) IsModelAvailable(ctx context.Context, modelName string) bool {
	return mm.ValidateModel(ctx, modelName) == nil
}

// PullModel downloads a model from Ollama with progress tracking
func (mm *ModelManager) PullModel(ctx context.Context, modelName string, progressFn func(ollama.PullProgress)) error {
	// Check if Ollama is available
	if err := mm.ollamaClient.Ping(ctx); err != nil {
		return fmt.Errorf("Ollama not available: %w", err)
	}

	// Start model pull
	if err := mm.ollamaClient.PullModel(ctx, modelName, progressFn); err != nil {
		return fmt.Errorf("failed to pull model %s: %w", modelName, err)
	}

	// Clear cache to force refresh on next request
	mm.ClearCache()
	
	log.Printf("Successfully pulled model: %s", modelName)
	return nil
}

// DeleteModel removes a model from Ollama
func (mm *ModelManager) DeleteModel(ctx context.Context, modelName string) error {
	// Check if Ollama is available
	if err := mm.ollamaClient.Ping(ctx); err != nil {
		return fmt.Errorf("Ollama not available: %w", err)
	}

	// Delete the model
	if err := mm.ollamaClient.DeleteModel(ctx, modelName); err != nil {
		return fmt.Errorf("failed to delete model %s: %w", modelName, err)
	}

	// Clear cache to force refresh on next request
	mm.ClearCache()
	
	log.Printf("Successfully deleted model: %s", modelName)
	return nil
}

// GetRunningModels returns currently running models from Ollama
func (mm *ModelManager) GetRunningModels(ctx context.Context) ([]ollama.RunningModel, error) {
	// Check if Ollama is available
	if err := mm.ollamaClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("Ollama not available: %w", err)
	}

	// Get running models
	runningModels, err := mm.ollamaClient.GetRunningModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get running models: %w", err)
	}

	return runningModels, nil
}