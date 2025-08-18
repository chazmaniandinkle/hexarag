package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"

	"github.com/username/hexarag/internal/domain/entities"
	"github.com/username/hexarag/internal/domain/ports"
	"github.com/username/hexarag/internal/domain/services"
)

// Adapter implements the LLMPort interface using OpenAI-compatible APIs
type Adapter struct {
	client       *openai.Client
	model        string
	baseURL      string
	provider     string
	modelManager *services.ModelManager
}

// NewAdapter creates a new OpenAI-compatible LLM adapter
func NewAdapter(baseURL, apiKey, model, provider string, modelManager *services.ModelManager) (*Adapter, error) {
	config := openai.DefaultConfig(apiKey)

	// Override base URL for local providers like Ollama/LM Studio
	if baseURL != "" {
		config.BaseURL = strings.TrimSuffix(baseURL, "/")
	}

	client := openai.NewClientWithConfig(config)

	return &Adapter{
		client:       client,
		model:        model,
		baseURL:      baseURL,
		provider:     provider,
		modelManager: modelManager,
	}, nil
}

// Complete generates a completion for the given messages
func (a *Adapter) Complete(ctx context.Context, request *ports.CompletionRequest) (*ports.CompletionResponse, error) {
	// Select and validate the model
	selectedModel := a.selectModel(request.Model)
	if err := a.validateModel(ctx, selectedModel); err != nil {
		return nil, fmt.Errorf("model validation failed: %w", err)
	}

	// Convert our messages to OpenAI format
	messages := a.convertMessages(request.Messages, request.SystemPrompt)

	// Build the request
	req := openai.ChatCompletionRequest{
		Model:       selectedModel,
		Messages:    messages,
		MaxTokens:   request.MaxTokens,
		Temperature: float32(request.Temperature),
		Stream:      false,
	}

	// Add tools if provided
	if len(request.Tools) > 0 {
		req.Tools = a.convertTools(request.Tools)
		if request.ToolChoice != nil {
			req.ToolChoice = request.ToolChoice
		}
	}

	// Make the API call
	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from API")
	}

	choice := resp.Choices[0]

	// Create response message
	responseMsg := &entities.Message{
		Role:    entities.RoleAssistant,
		Content: choice.Message.Content,
		Model:   resp.Model,
	}

	// Handle tool calls if present
	var toolCalls []*entities.ToolCall
	if len(choice.Message.ToolCalls) > 0 {
		for _, tc := range choice.Message.ToolCalls {
			toolCall := &entities.ToolCall{
				ID:     tc.ID,
				Name:   tc.Function.Name,
				Status: entities.ToolCallStatusPending,
			}

			// Parse arguments
			if tc.Function.Arguments != "" {
				args := make(map[string]interface{})
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
					toolCall.Arguments = args
				}
			}

			toolCalls = append(toolCalls, toolCall)
		}
	}

	// Create response
	response := &ports.CompletionResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		Message:      responseMsg,
		FinishReason: string(choice.FinishReason),
		ToolCalls:    toolCalls,
	}

	// Add usage information if available
	if resp.Usage.TotalTokens > 0 {
		response.Usage = &ports.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return response, nil
}

// CompleteStream generates a streaming completion
func (a *Adapter) CompleteStream(ctx context.Context, request *ports.CompletionRequest, handler ports.StreamHandler) error {
	// Select and validate the model
	selectedModel := a.selectModel(request.Model)
	if err := a.validateModel(ctx, selectedModel); err != nil {
		return fmt.Errorf("model validation failed: %w", err)
	}

	// Convert our messages to OpenAI format
	messages := a.convertMessages(request.Messages, request.SystemPrompt)

	// Build the request
	req := openai.ChatCompletionRequest{
		Model:       selectedModel,
		Messages:    messages,
		MaxTokens:   request.MaxTokens,
		Temperature: float32(request.Temperature),
		Stream:      true,
	}

	// Add tools if provided
	if len(request.Tools) > 0 {
		req.Tools = a.convertTools(request.Tools)
		if request.ToolChoice != nil {
			req.ToolChoice = request.ToolChoice
		}
	}

	// Create streaming request
	stream, err := a.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create streaming completion: %w", err)
	}
	defer stream.Close()

	// Process stream
	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Send final chunk
				return handler(&ports.StreamChunk{
					Done: true,
				})
			}
			return fmt.Errorf("streaming error: %w", err)
		}

		if len(response.Choices) > 0 {
			choice := response.Choices[0]
			chunk := &ports.StreamChunk{
				ID:           response.ID,
				Delta:        choice.Delta.Content,
				FinishReason: string(choice.FinishReason),
				Done:         choice.FinishReason != "",
			}

			// Handle tool calls in streaming
			if len(choice.Delta.ToolCalls) > 0 {
				var tools []ports.Tool
				for _, tc := range choice.Delta.ToolCalls {
					tool := ports.Tool{
						Type: "function",
						Function: ports.ToolFunction{
							Name: tc.Function.Name,
						},
					}
					tools = append(tools, tool)
				}
				chunk.ToolCalls = tools
			}

			if err := handler(chunk); err != nil {
				return fmt.Errorf("stream handler error: %w", err)
			}

			if chunk.Done {
				break
			}
		}
	}

	return nil
}

// CountTokens counts the tokens in the given text
func (a *Adapter) CountTokens(ctx context.Context, text string) (int, error) {
	// For local providers, we need to estimate since they don't always provide tokenization endpoints
	if a.isLocalProvider() {
		return a.estimateTokens(text), nil
	}

	// For OpenAI and compatible services, try to use their tokenization
	// This is a simplified approach - in production, you might want to use tiktoken
	// or the provider's specific tokenization endpoint
	return a.estimateTokens(text), nil
}

// estimateTokens provides a rough token count estimation
func (a *Adapter) estimateTokens(text string) int {
	// Rough estimation: ~4 characters per token for English text
	// This is a simplified approach - for more accuracy, use proper tokenization libraries
	words := len(strings.Fields(text))
	chars := len(text)

	// Use the higher of word-based or character-based estimation
	wordBasedTokens := int(float64(words) * 1.3) // ~1.3 tokens per word
	charBasedTokens := chars / 4                 // ~4 chars per token

	if wordBasedTokens > charBasedTokens {
		return wordBasedTokens
	}
	return charBasedTokens
}

// GetModels returns the list of available models
func (a *Adapter) GetModels(ctx context.Context) ([]ports.Model, error) {
	if a.isLocalProvider() && a.modelManager != nil {
		// For local providers with ModelManager, use real Ollama data
		return a.modelManager.GetAvailableModels(ctx)
	}

	if a.isLocalProvider() {
		// Fallback for local providers without ModelManager
		return []ports.Model{
			{
				ID:          a.model,
				Name:        a.model,
				Description: fmt.Sprintf("Local model via %s", a.provider),
				Available:   true,
			},
		}, nil
	}

	// For OpenAI and compatible APIs, fetch available models
	modelsList, err := a.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	var models []ports.Model
	for _, model := range modelsList.Models {
		models = append(models, ports.Model{
			ID:        model.ID,
			Name:      model.ID,
			Available: true,
		})
	}

	return models, nil
}

// Ping checks LLM connectivity
func (a *Adapter) Ping(ctx context.Context) error {
	// Try to get models list as a connectivity test
	_, err := a.GetModels(ctx)
	if err != nil {
		return fmt.Errorf("LLM ping failed: %w", err)
	}
	return nil
}

// Helper methods

// convertMessages converts our domain messages to OpenAI format
func (a *Adapter) convertMessages(messages []*entities.Message, systemPrompt string) []openai.ChatCompletionMessage {
	var result []openai.ChatCompletionMessage

	// Add system prompt if provided
	if systemPrompt != "" {
		result = append(result, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}

	// Convert domain messages
	for _, msg := range messages {
		oaiMsg := openai.ChatCompletionMessage{
			Role:    a.convertRole(msg.Role),
			Content: msg.Content,
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			var toolCalls []openai.ToolCall
			for _, tc := range msg.ToolCalls {
				args, _ := json.Marshal(tc.Arguments)
				toolCalls = append(toolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(args),
					},
				})
			}
			oaiMsg.ToolCalls = toolCalls
		}

		result = append(result, oaiMsg)
	}

	return result
}

// convertRole converts our domain roles to OpenAI roles
func (a *Adapter) convertRole(role entities.MessageRole) string {
	switch role {
	case entities.RoleUser:
		return openai.ChatMessageRoleUser
	case entities.RoleAssistant:
		return openai.ChatMessageRoleAssistant
	case entities.RoleSystem:
		return openai.ChatMessageRoleSystem
	case entities.RoleTool:
		return openai.ChatMessageRoleTool
	default:
		return openai.ChatMessageRoleUser
	}
}

// convertTools converts our domain tools to OpenAI format
func (a *Adapter) convertTools(tools []ports.Tool) []openai.Tool {
	var result []openai.Tool
	for _, tool := range tools {
		result = append(result, openai.Tool{
			Type: openai.ToolType(tool.Type),
			Function: &openai.FunctionDefinition{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}
	return result
}

// selectModel chooses which model to use for the request
func (a *Adapter) selectModel(requestModel string) string {
	if requestModel != "" {
		return requestModel
	}
	return a.model
}

// validateModel checks if a model is available (for local providers with ModelManager)
func (a *Adapter) validateModel(ctx context.Context, modelName string) error {
	if a.isLocalProvider() && a.modelManager != nil {
		return a.modelManager.ValidateModel(ctx, modelName)
	}
	// For non-local providers, assume model is valid
	return nil
}

// isLocalProvider checks if we're using a local provider
func (a *Adapter) isLocalProvider() bool {
	return strings.Contains(a.baseURL, "localhost") ||
		strings.Contains(a.baseURL, "127.0.0.1") ||
		a.provider == "ollama" ||
		a.provider == "lm-studio"
}
