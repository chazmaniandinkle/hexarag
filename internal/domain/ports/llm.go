package ports

import (
	"context"
	"time"

	"hexarag/internal/domain/entities"
)

// LLMPort defines the interface for Language Model operations
type LLMPort interface {
	// Complete generates a completion for the given messages
	Complete(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error)

	// CompleteStream generates a streaming completion
	CompleteStream(ctx context.Context, request *CompletionRequest, handler StreamHandler) error

	// CountTokens counts the tokens in the given text
	CountTokens(ctx context.Context, text string) (int, error)

	// GetModels returns the list of available models
	GetModels(ctx context.Context) ([]Model, error)

	// Health check
	Ping(ctx context.Context) error
}

// CompletionRequest represents a request to generate a completion
type CompletionRequest struct {
	Messages     []*entities.Message `json:"messages"`
	Model        string              `json:"model"`
	MaxTokens    int                 `json:"max_tokens,omitempty"`
	Temperature  float64             `json:"temperature,omitempty"`
	Tools        []Tool              `json:"tools,omitempty"`
	ToolChoice   interface{}         `json:"tool_choice,omitempty"`
	SystemPrompt string              `json:"system_prompt,omitempty"`
	Stream       bool                `json:"stream,omitempty"`
}

// CompletionResponse represents the response from a completion request
type CompletionResponse struct {
	ID           string               `json:"id"`
	Model        string               `json:"model"`
	Message      *entities.Message    `json:"message"`
	FinishReason string               `json:"finish_reason"`
	Usage        *TokenUsage          `json:"usage,omitempty"`
	ToolCalls    []*entities.ToolCall `json:"tool_calls,omitempty"`
}

// StreamHandler defines a function type for handling streaming responses
type StreamHandler func(chunk *StreamChunk) error

// StreamChunk represents a chunk of streaming response
type StreamChunk struct {
	ID           string `json:"id"`
	Delta        string `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
	ToolCalls    []Tool `json:"tool_calls,omitempty"`
	Done         bool   `json:"done"`
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Model represents an available language model
type Model struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Size        int64     `json:"size,omitempty"`
	Family      string    `json:"family,omitempty"`
	Parameters  string    `json:"parameters,omitempty"`
	Available   bool      `json:"available"`
	ModifiedAt  time.Time `json:"modified_at,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
