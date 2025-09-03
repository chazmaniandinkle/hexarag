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

// MessageFlowOrchestrator coordinates the flow of events between services
// It handles the orchestration of message processing from context construction to inference
type MessageFlowOrchestrator struct {
	storage   ports.StoragePort
	messaging ports.MessagingPort
	config    *FlowOrchestratorConfig
}

// FlowOrchestratorConfig holds configuration for the orchestrator
type FlowOrchestratorConfig struct {
	DefaultModel       string
	DefaultMaxTokens   int
	DefaultTemperature float64
	EnableTools        bool
	Timeout            time.Duration
}

// NewMessageFlowOrchestrator creates a new message flow orchestrator
func NewMessageFlowOrchestrator(storage ports.StoragePort, messaging ports.MessagingPort, config *FlowOrchestratorConfig) *MessageFlowOrchestrator {
	if config == nil {
		config = &FlowOrchestratorConfig{
			DefaultModel:       "qwen3:0.6b",
			DefaultMaxTokens:   4096,
			DefaultTemperature: 0.7,
			EnableTools:        true,
			Timeout:            30 * time.Second,
		}
	}

	return &MessageFlowOrchestrator{
		storage:   storage,
		messaging: messaging,
		config:    config,
	}
}

// StartListening starts the orchestrator by subscribing to relevant events
func (mfo *MessageFlowOrchestrator) StartListening(ctx context.Context) error {
	// Subscribe to context ready events to trigger inference
	err := mfo.messaging.SubscribeQueue(ctx, ports.SubjectContextReady, "message-flow-orchestrator", mfo.handleContextReady)
	if err != nil {
		return fmt.Errorf("failed to subscribe to context ready events: %w", err)
	}

	// Subscribe to inference response events for completion handling
	err = mfo.messaging.SubscribeQueue(ctx, ports.SubjectInferenceResponse, "message-flow-orchestrator", mfo.handleInferenceResponse)
	if err != nil {
		return fmt.Errorf("failed to subscribe to inference response events: %w", err)
	}

	log.Println("Message Flow Orchestrator service started and listening for events")
	return nil
}

// handleContextReady processes context ready events and triggers inference
func (mfo *MessageFlowOrchestrator) handleContextReady(ctx context.Context, subject string, data []byte) error {
	var contextResponse ContextResponse
	if err := json.Unmarshal(data, &contextResponse); err != nil {
		return fmt.Errorf("failed to unmarshal context response: %w", err)
	}

	log.Printf("Orchestrating inference request for conversation %s", contextResponse.ConversationID)

	// Get conversation details to determine model and settings
	conversation, err := mfo.storage.GetConversation(ctx, contextResponse.ConversationID)
	if err != nil {
		return fmt.Errorf("failed to get conversation for inference orchestration: %w", err)
	}

	// Create inference request from context response
	inferenceRequest := &InferenceRequest{
		ConversationID: contextResponse.ConversationID,
		MessageID:      contextResponse.MessageID,
		SystemPrompt:   contextResponse.SystemPrompt,
		Messages:       contextResponse.Messages,
		Model:          mfo.determineModel(conversation),
		MaxTokens:      mfo.determineMaxTokens(&contextResponse),
		Temperature:    mfo.config.DefaultTemperature,
		EnableTools:    mfo.config.EnableTools,
	}

	// Publish inference request
	if err := mfo.messaging.PublishJSON(ctx, ports.SubjectInferenceRequest, inferenceRequest); err != nil {
		// Publish error event
		errorEvent := map[string]interface{}{
			"error":           fmt.Sprintf("Failed to publish inference request: %v", err),
			"conversation_id": contextResponse.ConversationID,
			"message_id":      contextResponse.MessageID,
			"timestamp":       time.Now(),
			"orchestrator":    "message_flow",
		}
		mfo.messaging.PublishJSON(ctx, ports.SubjectSystemError, errorEvent)
		return fmt.Errorf("failed to publish inference request: %w", err)
	}

	log.Printf("Inference request published for conversation %s", contextResponse.ConversationID)
	return nil
}

// handleInferenceResponse processes inference completion events
func (mfo *MessageFlowOrchestrator) handleInferenceResponse(ctx context.Context, subject string, data []byte) error {
	var inferenceResponse InferenceResponse
	if err := json.Unmarshal(data, &inferenceResponse); err != nil {
		return fmt.Errorf("failed to unmarshal inference response: %w", err)
	}

	log.Printf("Processing inference completion for conversation %s (finish_reason: %s)",
		inferenceResponse.ConversationID, inferenceResponse.FinishReason)

	// Update conversation with the response message
	conversation, err := mfo.storage.GetConversation(ctx, inferenceResponse.ConversationID)
	if err != nil {
		return fmt.Errorf("failed to get conversation for completion handling: %w", err)
	}

	// Add the response message to conversation
	if inferenceResponse.ResponseMessage != nil {
		conversation.AddMessage(inferenceResponse.ResponseMessage.ID)
		if err := mfo.storage.UpdateConversation(ctx, conversation); err != nil {
			log.Printf("Warning: failed to update conversation with response message: %v", err)
		}
	}

	// Publish conversation completion event for WebSocket notifications
	completionEvent := map[string]interface{}{
		"conversation_id":  inferenceResponse.ConversationID,
		"message_id":       inferenceResponse.MessageID,
		"response_message": inferenceResponse.ResponseMessage,
		"finish_reason":    inferenceResponse.FinishReason,
		"processing_time":  inferenceResponse.ProcessingTime.Milliseconds(),
		"token_usage":      inferenceResponse.TokenUsage,
		"completed_at":     time.Now(),
	}

	// Use conversation-specific subject for targeted notifications
	completionSubject := fmt.Sprintf(ports.SubjectConversationMessageNew, inferenceResponse.ConversationID)
	if err := mfo.messaging.PublishJSON(ctx, completionSubject, completionEvent); err != nil {
		log.Printf("Warning: failed to publish completion event: %v", err)
	}

	return nil
}

// determineModel selects the appropriate model for inference
func (mfo *MessageFlowOrchestrator) determineModel(conversation *entities.Conversation) string {
	// In the future, this could be conversation-specific or user-preference based
	// For now, use the default model from configuration
	return mfo.config.DefaultModel
}

// determineMaxTokens calculates appropriate max tokens for inference
func (mfo *MessageFlowOrchestrator) determineMaxTokens(contextResponse *ContextResponse) int {
	// Reserve tokens for the response based on context size
	// This ensures we don't exceed model limits
	usedTokens := contextResponse.TokenCount
	remainingTokens := mfo.config.DefaultMaxTokens - usedTokens

	// Ensure minimum response tokens (at least 512 tokens for response)
	minResponseTokens := 512
	if remainingTokens < minResponseTokens {
		return minResponseTokens
	}

	// Cap the response to reasonable size (max 2048 tokens for response)
	maxResponseTokens := 2048
	if remainingTokens > maxResponseTokens {
		return maxResponseTokens
	}

	return remainingTokens
}

// GetFlowStatus provides status information about message flow orchestration
func (mfo *MessageFlowOrchestrator) GetFlowStatus(ctx context.Context) (map[string]interface{}, error) {
	status := map[string]interface{}{
		"service":             "message_flow_orchestrator",
		"status":              "active",
		"default_model":       mfo.config.DefaultModel,
		"default_max_tokens":  mfo.config.DefaultMaxTokens,
		"default_temperature": mfo.config.DefaultTemperature,
		"tools_enabled":       mfo.config.EnableTools,
		"timeout":             mfo.config.Timeout.String(),
		"timestamp":           time.Now(),
	}

	// Test messaging connectivity
	if err := mfo.messaging.Ping(); err != nil {
		status["messaging_status"] = "error"
		status["messaging_error"] = err.Error()
	} else {
		status["messaging_status"] = "healthy"
	}

	// Test storage connectivity
	if err := mfo.storage.Ping(ctx); err != nil {
		status["storage_status"] = "error"
		status["storage_error"] = err.Error()
	} else {
		status["storage_status"] = "healthy"
	}

	return status, nil
}

// UpdateConfiguration updates the orchestrator configuration
func (mfo *MessageFlowOrchestrator) UpdateConfiguration(config *FlowOrchestratorConfig) {
	if config != nil {
		mfo.config = config
		log.Printf("Message Flow Orchestrator configuration updated: model=%s, max_tokens=%d, temperature=%.2f",
			config.DefaultModel, config.DefaultMaxTokens, config.DefaultTemperature)
	}
}
