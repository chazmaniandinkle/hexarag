package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "basic URL",
			baseURL:     "http://localhost:11434",
			expectedURL: "http://localhost:11434",
		},
		{
			name:        "URL with /v1 suffix",
			baseURL:     "http://localhost:11434/v1",
			expectedURL: "http://localhost:11434",
		},
		{
			name:        "URL with trailing slash",
			baseURL:     "http://localhost:11434/",
			expectedURL: "http://localhost:11434",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL)
			if client.baseURL != tt.expectedURL {
				t.Errorf("NewClient() baseURL = %v, want %v", client.baseURL, tt.expectedURL)
			}
		})
	}
}

func TestClient_ListModels(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("Expected path /api/tags, got %s", r.URL.Path)
		}

		response := ListModelsResponse{
			Models: []Model{
				{
					Name:       "deepseek-r1:8b",
					ModifiedAt: time.Now(),
					Size:       5000000000,
					Digest:     "sha256:abc123",
				},
				{
					Name:       "llama3.2:3b",
					ModifiedAt: time.Now(),
					Size:       3000000000,
					Digest:     "sha256:def456",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	models, err := client.ListModels(context.Background())

	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	if models[0].Name != "deepseek-r1:8b" {
		t.Errorf("Expected first model to be 'deepseek-r1:8b', got %s", models[0].Name)
	}
}

func TestClient_ListModels_ErrorResponse(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.ListModels(context.Background())

	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestClient_Ping(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectError    bool
	}{
		{
			name:        "successful ping",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "server error",
			statusCode:  http.StatusInternalServerError,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					// Return a minimal valid response for successful ping
					response := ListModelsResponse{Models: []Model{}}
					json.NewEncoder(w).Encode(response)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL)
			err := client.Ping(context.Background())

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestClient_ShowModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" {
			t.Errorf("Expected path /api/show, got %s", r.URL.Path)
		}

		// Decode request to check model name
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		if req["name"] != "test-model" {
			t.Errorf("Expected model name 'test-model', got %s", req["name"])
		}

		response := ModelInfo{
			License:   "MIT",
			Modelfile: "FROM llama2",
			Details: ModelDetails{
				Format:            "gguf",
				Family:            "llama",
				ParameterSize:     "7B",
				QuantizationLevel: "Q4_0",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	info, err := client.ShowModel(context.Background(), "test-model")

	if err != nil {
		t.Fatalf("ShowModel() error = %v", err)
	}

	if info.Details.Family != "llama" {
		t.Errorf("Expected family 'llama', got %s", info.Details.Family)
	}
}