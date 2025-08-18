package services

import (
	"context"
	"testing"
	"time"

	"github.com/username/hexarag/internal/adapters/llm/ollama"
	"github.com/username/hexarag/internal/domain/ports"
)

// MockOllamaClient implements a mock Ollama client for testing
type MockOllamaClient struct {
	models    []ollama.Model
	available bool
	err       error
}

// Ensure MockOllamaClient implements OllamaClientInterface
var _ OllamaClientInterface = (*MockOllamaClient)(nil)

func (m *MockOllamaClient) ListModels(ctx context.Context) ([]ollama.Model, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.models, nil
}

func (m *MockOllamaClient) ShowModel(ctx context.Context, name string) (*ollama.ModelInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ollama.ModelInfo{
		Details: ollama.ModelDetails{
			Family:        "deepseek",
			ParameterSize: "8B",
		},
	}, nil
}

func (m *MockOllamaClient) PullModel(ctx context.Context, name string, progressFn func(ollama.PullProgress)) error {
	if m.err != nil {
		return m.err
	}
	// Simulate progress callback
	if progressFn != nil {
		progressFn(ollama.PullProgress{
			Status:    "downloading",
			Completed: 50,
			Total:     100,
		})
	}
	return nil
}

func (m *MockOllamaClient) DeleteModel(ctx context.Context, name string) error {
	return m.err
}

func (m *MockOllamaClient) GetRunningModels(ctx context.Context) ([]ollama.RunningModel, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return empty slice instead of nil
	return []ollama.RunningModel{}, nil
}

func (m *MockOllamaClient) Ping(ctx context.Context) error {
	if !m.available {
		return context.DeadlineExceeded
	}
	return m.err
}

func TestModelManager_GetAvailableModels(t *testing.T) {
	mockClient := &MockOllamaClient{
		models: []ollama.Model{
			{
				Name:       "deepseek-r1:8b",
				ModifiedAt: time.Now(),
				Size:       5000000000,
				Details: &ollama.ModelDetails{
					Family:        "deepseek",
					ParameterSize: "8B",
				},
			},
		},
		available: true,
	}

	// Create a custom ModelManager with a shorter cache TTL for testing
	mm := &ModelManager{
		ollamaClient: mockClient,
		cache: &ModelCache{
			models:    nil,
			timestamp: time.Time{},
			ttl:       1 * time.Millisecond, // Very short TTL for testing
		},
	}

	models, err := mm.GetAvailableModels(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableModels() error = %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	if models[0].ID != "deepseek-r1:8b" {
		t.Errorf("Expected model ID 'deepseek-r1:8b', got %s", models[0].ID)
	}

	if models[0].Family != "deepseek" {
		t.Errorf("Expected family 'deepseek', got %s", models[0].Family)
	}
}

func TestModelManager_GetAvailableModels_CacheHit(t *testing.T) {
	mockClient := &MockOllamaClient{
		available: true,
	}

	mm := NewModelManager(mockClient)

	// Pre-populate cache with proper ports.Model type
	cachedModels := []ports.Model{
		{
			ID:        "cached-model",
			Name:      "Cached Model",
			Available: true,
		},
	}

	mm.cache.models = cachedModels
	mm.cache.timestamp = time.Now()

	// This should return cached results without calling Ollama
	models, err := mm.GetAvailableModels(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableModels() error = %v", err)
	}

	// Should return cached result
	if len(models) != 1 {
		t.Errorf("Expected 1 cached model, got %d", len(models))
	}

	if models[0].ID != "cached-model" {
		t.Errorf("Expected cached model ID 'cached-model', got %s", models[0].ID)
	}
}

func TestModelManager_GetAvailableModels_Fallback(t *testing.T) {
	mockClient := &MockOllamaClient{
		available: false, // Ollama not available
	}

	mm := NewModelManager(mockClient)
	models, err := mm.GetAvailableModels(context.Background())

	if err != nil {
		t.Fatalf("GetAvailableModels() error = %v", err)
	}

	// Should return fallback models
	if len(models) == 0 {
		t.Error("Expected fallback models, got empty result")
	}

	// Check that fallback model is marked as unavailable
	found := false
	for _, model := range models {
		if model.ID == "deepseek-r1:8b" && !model.Available {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find unavailable deepseek-r1:8b in fallback models")
	}
}

func TestModelManager_ValidateModel(t *testing.T) {
	tests := []struct {
		name        string
		modelName   string
		available   bool
		expectError bool
	}{
		{
			name:        "valid model",
			modelName:   "deepseek-r1:8b",
			available:   true,
			expectError: false,
		},
		{
			name:        "invalid model",
			modelName:   "nonexistent-model",
			available:   true,
			expectError: true,
		},
		{
			name:        "ollama unavailable",
			modelName:   "nonexistent-model",
			available:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockOllamaClient{
				models: []ollama.Model{
					{Name: "deepseek-r1:8b"},
				},
				available: tt.available,
			}

			mm := NewModelManager(mockClient)
			err := mm.ValidateModel(context.Background(), tt.modelName)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestModelManager_extractModelFamily(t *testing.T) {
	mm := NewModelManager(nil)

	tests := []struct {
		name     string
		expected string
	}{
		{"llama3.2:3b", "llama"},
		{"deepseek-r1:8b", "deepseek"},
		{"mistral:7b", "mistral"},
		{"phi3:mini", "phi"},
		{"qwen2.5:7b", "qwen"},
		{"gemma:2b", "gemma"},
		{"unknown-model", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mm.extractModelFamily(tt.name)
			if result != tt.expected {
				t.Errorf("extractModelFamily(%s) = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestModelManager_extractParameters(t *testing.T) {
	mm := NewModelManager(nil)

	tests := []struct {
		name     string
		expected string
	}{
		{"llama3.2:3b", "3B"},
		{"deepseek-r1:8b", "8B"},
		{"mistral:7b", "7B"},
		{"llama2:13b", "13B"},
		{"codestral:22b", "Unknown"},
		{"unknown-model", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mm.extractParameters(tt.name)
			if result != tt.expected {
				t.Errorf("extractParameters(%s) = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestModelManager_PullModel(t *testing.T) {
	tests := []struct {
		name        string
		available   bool
		expectError bool
	}{
		{
			name:        "successful pull",
			available:   true,
			expectError: false,
		},
		{
			name:        "ollama unavailable",
			available:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockOllamaClient{
				available: tt.available,
			}

			mm := NewModelManager(mockClient)
			progressCalled := false
			progressFn := func(progress ollama.PullProgress) {
				progressCalled = true
			}

			err := mm.PullModel(context.Background(), "test-model", progressFn)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			// Progress function should be called for successful pulls
			if !tt.expectError && !progressCalled {
				t.Error("Expected progress function to be called")
			}
		})
	}
}

func TestModelManager_DeleteModel(t *testing.T) {
	tests := []struct {
		name        string
		available   bool
		expectError bool
	}{
		{
			name:        "successful delete",
			available:   true,
			expectError: false,
		},
		{
			name:        "ollama unavailable",
			available:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockOllamaClient{
				available: tt.available,
			}

			mm := NewModelManager(mockClient)
			err := mm.DeleteModel(context.Background(), "test-model")

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestModelManager_GetRunningModels(t *testing.T) {
	mockClient := &MockOllamaClient{
		available: true,
	}

	mm := NewModelManager(mockClient)
	models, err := mm.GetRunningModels(context.Background())

	if err != nil {
		t.Fatalf("GetRunningModels() error = %v", err)
	}

	// Should return empty list from mock
	if models == nil {
		t.Error("Expected non-nil models slice")
	}
}