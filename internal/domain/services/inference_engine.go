package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"hexarag/internal/domain/entities"
	"hexarag/internal/domain/ports"
)

// InferenceEngine orchestrates LLM inference and tool execution
type InferenceEngine struct {
	storage   ports.StoragePort
	messaging ports.MessagingPort
	llm       ports.LLMPort
	tools     ports.ToolPort
}

// NewInferenceEngine creates a new inference engine service
func NewInferenceEngine(storage ports.StoragePort, messaging ports.MessagingPort, llm ports.LLMPort, tools ports.ToolPort) *InferenceEngine {
	return &InferenceEngine{
		storage:   storage,
		messaging: messaging,
		llm:       llm,
		tools:     tools,
	}
}

// InferenceRequest represents a request for LLM inference
type InferenceRequest struct {
	ConversationID string              `json:"conversation_id"`
	MessageID      string              `json:"message_id"`
	SystemPrompt   string              `json:"system_prompt"`
	Messages       []*entities.Message `json:"messages"`
	Model          string              `json:"model,omitempty"`
	MaxTokens      int                 `json:"max_tokens,omitempty"`
	Temperature    float64             `json:"temperature,omitempty"`
	EnableTools    bool                `json:"enable_tools"`
}

// InferenceResponse represents the result of LLM inference
type InferenceResponse struct {
	ConversationID  string                 `json:"conversation_id"`
	MessageID       string                 `json:"message_id"`
	ResponseMessage *entities.Message      `json:"response_message"`
	ToolCalls       []*entities.ToolCall   `json:"tool_calls,omitempty"`
	FinishReason    string                 `json:"finish_reason"`
	TokenUsage      *ports.TokenUsage      `json:"token_usage,omitempty"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// StartListening starts the inference engine by subscribing to relevant events
func (ie *InferenceEngine) StartListening(ctx context.Context) error {
	// Subscribe to inference requests
	err := ie.messaging.SubscribeQueue(ctx, ports.SubjectInferenceRequest, "inference-engine", ie.handleInferenceRequest)
	if err != nil {
		return fmt.Errorf("failed to subscribe to inference requests: %w", err)
	}

	// Subscribe to tool execution results
	err = ie.messaging.SubscribeQueue(ctx, ports.SubjectToolResult, "inference-engine", ie.handleToolResult)
	if err != nil {
		return fmt.Errorf("failed to subscribe to tool results: %w", err)
	}

	log.Println("Inference Engine service started and listening for events")
	return nil
}

// handleInferenceRequest processes incoming inference requests
func (ie *InferenceEngine) handleInferenceRequest(ctx context.Context, subject string, data []byte) error {
	var request InferenceRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to unmarshal inference request: %w", err)
	}

	log.Printf("Processing inference request for conversation %s", request.ConversationID)

	// Execute inference
	response, err := ie.ExecuteInference(ctx, &request)
	if err != nil {
		log.Printf("Failed to execute inference for conversation %s: %v", request.ConversationID, err)

		// Publish error event
		errorEvent := map[string]interface{}{
			"error":           err.Error(),
			"conversation_id": request.ConversationID,
			"message_id":      request.MessageID,
			"timestamp":       time.Now(),
		}
		ie.messaging.PublishJSON(ctx, ports.SubjectSystemError, errorEvent)
		return err
	}

	// Publish the inference response
	err = ie.messaging.PublishJSON(ctx, ports.SubjectInferenceResponse, response)
	if err != nil {
		return fmt.Errorf("failed to publish inference response: %w", err)
	}

	log.Printf("Inference completed for conversation %s (finish_reason: %s)",
		request.ConversationID, response.FinishReason)

	return nil
}

// ExecuteInference performs LLM inference with optional tool calling
func (ie *InferenceEngine) ExecuteInference(ctx context.Context, request *InferenceRequest) (*InferenceResponse, error) {
	startTime := time.Now()

	// Build completion request
	completionRequest := &ports.CompletionRequest{
		Messages:     request.Messages,
		Model:        request.Model,
		MaxTokens:    request.MaxTokens,
		Temperature:  request.Temperature,
		SystemPrompt: request.SystemPrompt,
	}

	// Add tools if enabled
	if request.EnableTools {
		tools, err := ie.tools.GetAvailableTools(ctx)
		if err != nil {
			log.Printf("Warning: failed to get available tools: %v", err)
		} else {
			completionRequest.Tools = tools
		}
	}

	// Execute completion
	completionResponse, err := ie.llm.Complete(ctx, completionRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute LLM completion: %w", err)
	}

	// Create response message
	responseMessage := entities.NewMessage(request.ConversationID, entities.RoleAssistant, completionResponse.Message.Content)
	responseMessage.Model = completionResponse.Model
	responseMessage.TokenCount = 0 // Will be calculated by tokenizer if needed

	// Handle tool calls if present
	var toolCalls []*entities.ToolCall
	if len(completionResponse.ToolCalls) > 0 {
		toolCalls = completionResponse.ToolCalls

		// Associate tool calls with the response message
		for _, tc := range toolCalls {
			tc.MessageID = responseMessage.ID
			responseMessage.AddToolCall(*tc)
		}

		// Execute tool calls
		if err := ie.executeToolCalls(ctx, toolCalls); err != nil {
			log.Printf("Warning: tool execution failed: %v", err)
		}
	}

	// Save the response message
	if err := ie.storage.SaveMessage(ctx, responseMessage); err != nil {
		return nil, fmt.Errorf("failed to save response message: %w", err)
	}

	processingTime := time.Since(startTime)

	// Create inference response
	response := &InferenceResponse{
		ConversationID:  request.ConversationID,
		MessageID:       request.MessageID,
		ResponseMessage: responseMessage,
		ToolCalls:       toolCalls,
		FinishReason:    completionResponse.FinishReason,
		TokenUsage:      completionResponse.Usage,
		ProcessingTime:  processingTime,
		Metadata: map[string]interface{}{
			"model":              completionResponse.Model,
			"tools_enabled":      request.EnableTools,
			"tool_calls_count":   len(toolCalls),
			"processing_time_ms": processingTime.Milliseconds(),
			"completed_at":       time.Now(),
		},
	}

	return response, nil
}

// executeToolCalls executes tool calls asynchronously
func (ie *InferenceEngine) executeToolCalls(ctx context.Context, toolCalls []*entities.ToolCall) error {
	for _, toolCall := range toolCalls {
		// Publish tool execution request
		toolRequest := &ports.ToolExecutionRequest{
			ToolCallID:     toolCall.ID,
			Name:           toolCall.Name,
			Arguments:      toolCall.Arguments,
			MessageID:      toolCall.MessageID,
			ConversationID: "", // Will be determined by tool executor
		}

		if err := ie.messaging.PublishJSON(ctx, ports.SubjectToolExecute, toolRequest); err != nil {
			log.Printf("Failed to publish tool execution request for %s: %v", toolCall.Name, err)

			// Mark tool call as failed
			toolCall.SetError(fmt.Sprintf("Failed to execute tool: %v", err))
			ie.storage.UpdateToolCall(ctx, toolCall)
		}
	}

	return nil
}

// handleToolResult processes tool execution results
func (ie *InferenceEngine) handleToolResult(ctx context.Context, subject string, data []byte) error {
	var toolResponse ports.ToolExecutionResponse
	if err := json.Unmarshal(data, &toolResponse); err != nil {
		return fmt.Errorf("failed to unmarshal tool response: %w", err)
	}

	// Get the tool call
	toolCall, err := ie.storage.GetToolCall(ctx, toolResponse.ToolCallID)
	if err != nil {
		return fmt.Errorf("failed to get tool call: %w", err)
	}

	// Update tool call with result
	if toolResponse.Result.Success {
		toolCall.SetResult(toolResponse.Result.Data)
	} else {
		toolCall.SetError(toolResponse.Result.Error)
	}

	// Save updated tool call
	if err := ie.storage.UpdateToolCall(ctx, toolCall); err != nil {
		return fmt.Errorf("failed to update tool call: %w", err)
	}

	log.Printf("Tool call %s completed with status: %s", toolCall.Name, toolCall.Status)

	// Check if we need to continue inference (for multi-step tool calling)
	// This would be implemented in a more sophisticated version
	return nil
}

// ExecuteStreamingInference performs streaming LLM inference
func (ie *InferenceEngine) ExecuteStreamingInference(ctx context.Context, request *InferenceRequest, handler ports.StreamHandler) error {
	// Build completion request
	completionRequest := &ports.CompletionRequest{
		Messages:     request.Messages,
		Model:        request.Model,
		MaxTokens:    request.MaxTokens,
		Temperature:  request.Temperature,
		SystemPrompt: request.SystemPrompt,
		Stream:       true,
	}

	// Add tools if enabled
	if request.EnableTools {
		tools, err := ie.tools.GetAvailableTools(ctx)
		if err != nil {
			log.Printf("Warning: failed to get available tools: %v", err)
		} else {
			completionRequest.Tools = tools
		}
	}

	// Execute streaming completion
	return ie.llm.CompleteStream(ctx, completionRequest, handler)
}

// GetInferenceStatus returns current status of the inference engine
func (ie *InferenceEngine) GetInferenceStatus(ctx context.Context) (map[string]interface{}, error) {
	status := map[string]interface{}{
		"status":    "running",
		"timestamp": time.Now(),
	}

	// Check LLM connectivity
	if err := ie.llm.Ping(ctx); err != nil {
		status["llm_status"] = "error"
		status["llm_error"] = err.Error()
	} else {
		status["llm_status"] = "connected"
	}

	// Check tool connectivity
	if err := ie.tools.Ping(ctx); err != nil {
		status["tools_status"] = "error"
		status["tools_error"] = err.Error()
	} else {
		status["tools_status"] = "connected"

		// Get available tools
		tools, err := ie.tools.GetAvailableTools(ctx)
		if err == nil {
			status["available_tools"] = len(tools)
		}
	}

	// Get LLM models if available
	models, err := ie.llm.GetModels(ctx)
	if err == nil {
		status["available_models"] = len(models)
	}

	return status, nil
}
