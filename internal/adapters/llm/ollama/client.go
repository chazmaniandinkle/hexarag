package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client provides native Ollama API access
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Ollama API client
func NewClient(baseURL string) *Client {
	// Ensure baseURL doesn't end with /v1 suffix (we want raw Ollama API)
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Model represents an Ollama model
type Model struct {
	Name       string     `json:"name"`
	ModifiedAt time.Time  `json:"modified_at"`
	Size       int64      `json:"size"`
	Digest     string     `json:"digest"`
	Details    *ModelDetails `json:"details,omitempty"`
}

// ModelDetails contains detailed model information
type ModelDetails struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ModelInfo contains complete model information from show endpoint
type ModelInfo struct {
	License    string          `json:"license"`
	Modelfile  string          `json:"modelfile"`
	Parameters map[string]interface{} `json:"parameters"`
	Template   string          `json:"template"`
	Details    ModelDetails    `json:"details"`
}

// PullProgress represents download progress for a model
type PullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// RunningModel represents a currently loaded model
type RunningModel struct {
	Name      string    `json:"name"`
	Model     string    `json:"model"`
	Size      int64     `json:"size"`
	SizeVRAM  int64     `json:"size_vram"`
	Digest    string    `json:"digest"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ListModelsResponse is the response from /api/tags
type ListModelsResponse struct {
	Models []Model `json:"models"`
}

// RunningModelsResponse is the response from /api/ps
type RunningModelsResponse struct {
	Models []RunningModel `json:"models"`
}

// ListModels returns all available models
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response ListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Models, nil
}

// ShowModel gets detailed information about a specific model
func (c *Client) ShowModel(ctx context.Context, name string) (*ModelInfo, error) {
	reqBody := map[string]string{"name": name}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/show", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var modelInfo ModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&modelInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &modelInfo, nil
}

// PullModel downloads a model with progress callback
func (c *Client) PullModel(ctx context.Context, name string, progressFn func(PullProgress)) error {
	reqBody := map[string]string{"name": name}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/pull", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Ollama returns streaming JSON responses for pull operations
	decoder := json.NewDecoder(resp.Body)
	for {
		var progress PullProgress
		if err := decoder.Decode(&progress); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode progress: %w", err)
		}

		if progressFn != nil {
			progressFn(progress)
		}

		// Check if pull is complete
		if progress.Status == "success" {
			break
		}
	}

	return nil
}

// DeleteModel removes a model
func (c *Client) DeleteModel(ctx context.Context, name string) error {
	reqBody := map[string]string{"name": name}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/delete", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetRunningModels returns currently loaded models
func (c *Client) GetRunningModels(ctx context.Context) ([]RunningModel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/ps", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response RunningModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Models, nil
}

// Ping checks if Ollama is available
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	return nil
}