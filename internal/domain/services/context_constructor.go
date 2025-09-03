package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"hexarag/internal/domain/entities"
	"hexarag/internal/domain/ports"
	"hexarag/pkg/tokenizer"
)

// ContextConstructor is the core service responsible for building rich context for LLM inference
type ContextConstructor struct {
	storage   ports.StoragePort
	messaging ports.MessagingPort
	tokenizer *tokenizer.Tokenizer
	maxTokens int
}

// NewContextConstructor creates a new context constructor service
func NewContextConstructor(storage ports.StoragePort, messaging ports.MessagingPort, model string, maxTokens int) (*ContextConstructor, error) {
	tokenizer, err := tokenizer.NewTokenizer(model)
	if err != nil {
		return nil, fmt.Errorf("failed to create tokenizer: %w", err)
	}

	return &ContextConstructor{
		storage:   storage,
		messaging: messaging,
		tokenizer: tokenizer,
		maxTokens: maxTokens,
	}, nil
}

// ContextRequest represents a request to build context for a conversation
type ContextRequest struct {
	ConversationID       string `json:"conversation_id"`
	MessageID            string `json:"message_id"`
	UseExtendedKnowledge bool   `json:"use_extended_knowledge"`
	MaxContextTokens     int    `json:"max_context_tokens,omitempty"`
}

// ContextResponse represents the constructed context for inference
type ContextResponse struct {
	ConversationID   string                 `json:"conversation_id"`
	MessageID        string                 `json:"message_id"`
	SystemPrompt     string                 `json:"system_prompt"`
	Messages         []*entities.Message    `json:"messages"`
	TokenCount       int                    `json:"token_count"`
	TruncatedHistory bool                   `json:"truncated_history"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// StartListening starts the context constructor service by subscribing to relevant events
func (cc *ContextConstructor) StartListening(ctx context.Context) error {
	// Subscribe to context requests
	err := cc.messaging.SubscribeQueue(ctx, ports.SubjectContextRequest, "context-constructor", cc.handleContextRequest)
	if err != nil {
		return fmt.Errorf("failed to subscribe to context requests: %w", err)
	}

	log.Println("Context Constructor service started and listening for events")
	return nil
}

// handleContextRequest processes incoming context construction requests
func (cc *ContextConstructor) handleContextRequest(ctx context.Context, subject string, data []byte) error {
	var request ContextRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to unmarshal context request: %w", err)
	}

	log.Printf("Processing context request for conversation %s", request.ConversationID)

	// Build the context
	response, err := cc.BuildContext(ctx, &request)
	if err != nil {
		log.Printf("Failed to build context for conversation %s: %v", request.ConversationID, err)

		// Publish error event
		errorEvent := map[string]interface{}{
			"error":           err.Error(),
			"conversation_id": request.ConversationID,
			"message_id":      request.MessageID,
			"timestamp":       time.Now(),
		}
		cc.messaging.PublishJSON(ctx, ports.SubjectSystemError, errorEvent)
		return err
	}

	// Publish the constructed context
	err = cc.messaging.PublishJSON(ctx, ports.SubjectContextReady, response)
	if err != nil {
		return fmt.Errorf("failed to publish context response: %w", err)
	}

	log.Printf("Context built successfully for conversation %s (%d tokens)",
		request.ConversationID, response.TokenCount)

	return nil
}

// BuildContext constructs context for a conversation
func (cc *ContextConstructor) BuildContext(ctx context.Context, request *ContextRequest) (*ContextResponse, error) {
	// Get conversation details
	conversation, err := cc.storage.GetConversation(ctx, request.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Get system prompt
	systemPrompt, err := cc.storage.GetSystemPrompt(ctx, conversation.SystemPromptID)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}

	// Determine maximum context tokens
	maxContextTokens := cc.maxTokens
	if request.MaxContextTokens > 0 && request.MaxContextTokens < maxContextTokens {
		maxContextTokens = request.MaxContextTokens
	}

	// Reserve tokens for the system prompt
	systemPromptTokens := cc.tokenizer.CountTokens(systemPrompt.Content)
	availableTokens := maxContextTokens - systemPromptTokens

	// Get conversation messages with intelligent selection
	messages, truncated, err := cc.selectMessages(ctx, request.ConversationID, availableTokens, request.UseExtendedKnowledge)
	if err != nil {
		return nil, fmt.Errorf("failed to select messages: %w", err)
	}

	// Calculate final token count
	totalTokens := systemPromptTokens + cc.tokenizer.CountConversationTokens(messages, "")

	// Create response
	response := &ContextResponse{
		ConversationID:   request.ConversationID,
		MessageID:        request.MessageID,
		SystemPrompt:     systemPrompt.Content,
		Messages:         messages,
		TokenCount:       totalTokens,
		TruncatedHistory: truncated,
		Metadata: map[string]interface{}{
			"system_prompt_id":       conversation.SystemPromptID,
			"system_prompt_tokens":   systemPromptTokens,
			"message_count":          len(messages),
			"total_messages":         len(conversation.MessageIDs),
			"use_extended_knowledge": request.UseExtendedKnowledge,
			"max_context_tokens":     maxContextTokens,
			"constructed_at":         time.Now(),
		},
	}

	return response, nil
}

// selectMessages intelligently selects messages for context based on token limits
func (cc *ContextConstructor) selectMessages(ctx context.Context, conversationID string, availableTokens int, useExtendedKnowledge bool) ([]*entities.Message, bool, error) {
	// For MVP, we'll use a simple strategy: get recent messages that fit in token limit
	// In future iterations, this could be enhanced with semantic search and relevance ranking

	if useExtendedKnowledge {
		return cc.selectMessagesWithExtendedKnowledge(ctx, conversationID, availableTokens)
	}

	return cc.selectRecentMessages(ctx, conversationID, availableTokens)
}

// selectRecentMessages selects the most recent messages that fit within token limits
func (cc *ContextConstructor) selectRecentMessages(ctx context.Context, conversationID string, availableTokens int) ([]*entities.Message, bool, error) {
	// Start with a reasonable batch size and expand if needed
	batchSize := 50

	messages, err := cc.storage.GetMessages(ctx, conversationID, batchSize)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		return []*entities.Message{}, false, nil
	}

	// Select messages from the end (most recent) backwards until we hit token limit
	var selectedMessages []*entities.Message
	var currentTokens int
	truncated := false

	// Iterate backwards through messages (from most recent)
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		messageTokens := cc.tokenizer.CountMessageTokens(message)

		if currentTokens+messageTokens > availableTokens {
			if len(selectedMessages) == 0 {
				// If even the first (most recent) message doesn't fit, truncate it
				message.Content = cc.tokenizer.TruncateToTokenLimit(message.Content, availableTokens-50) // Reserve 50 tokens for formatting
				selectedMessages = append(selectedMessages, message)
				truncated = true
			} else {
				truncated = true
			}
			break
		}

		// Add message to the beginning of selected messages (to maintain chronological order)
		selectedMessages = append([]*entities.Message{message}, selectedMessages...)
		currentTokens += messageTokens
	}

	return selectedMessages, truncated, nil
}

// selectMessagesWithExtendedKnowledge selects messages using extended knowledge (for future RAG enhancement)
func (cc *ContextConstructor) selectMessagesWithExtendedKnowledge(ctx context.Context, conversationID string, availableTokens int) ([]*entities.Message, bool, error) {
	// For MVP, this is the same as recent messages
	// In future phases, this would include:
	// 1. Semantic search for relevant historical messages
	// 2. Related conversation discovery
	// 3. External knowledge integration

	log.Printf("Using extended knowledge mode for conversation %s (MVP: same as recent messages)", conversationID)
	return cc.selectRecentMessages(ctx, conversationID, availableTokens)
}

// AnalyzeConversation provides insights about a conversation's context requirements
func (cc *ContextConstructor) AnalyzeConversation(ctx context.Context, conversationID string) (map[string]interface{}, error) {
	_, err := cc.storage.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Get all messages to analyze
	messages, err := cc.storage.GetMessages(ctx, conversationID, 1000) // Large limit to get all
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Calculate various metrics
	totalMessages := len(messages)
	totalTokens := cc.tokenizer.CountConversationTokens(messages, "")

	var userMessages, assistantMessages, toolMessages int
	var totalToolCalls int

	for _, msg := range messages {
		switch msg.Role {
		case entities.RoleUser:
			userMessages++
		case entities.RoleAssistant:
			assistantMessages++
		case entities.RoleTool:
			toolMessages++
		}
		totalToolCalls += len(msg.ToolCalls)
	}

	avgTokensPerMessage := 0
	if totalMessages > 0 {
		avgTokensPerMessage = totalTokens / totalMessages
	}

	analysis := map[string]interface{}{
		"conversation_id":           conversationID,
		"total_messages":            totalMessages,
		"total_tokens":              totalTokens,
		"avg_tokens_per_message":    avgTokensPerMessage,
		"user_messages":             userMessages,
		"assistant_messages":        assistantMessages,
		"tool_messages":             toolMessages,
		"total_tool_calls":          totalToolCalls,
		"fits_in_context":           totalTokens <= cc.maxTokens,
		"estimated_context_windows": (totalTokens + cc.maxTokens - 1) / cc.maxTokens, // Ceiling division
		"analyzed_at":               time.Now(),
	}

	if len(messages) > 0 {
		analysis["first_message_at"] = messages[0].CreatedAt
		analysis["last_message_at"] = messages[len(messages)-1].CreatedAt
		analysis["conversation_duration"] = messages[len(messages)-1].CreatedAt.Sub(messages[0].CreatedAt).String()
	}

	return analysis, nil
}

// UpdateTokenizer updates the tokenizer when the model changes
func (cc *ContextConstructor) UpdateTokenizer(model string) error {
	newTokenizer, err := tokenizer.NewTokenizer(model)
	if err != nil {
		return fmt.Errorf("failed to create new tokenizer: %w", err)
	}

	cc.tokenizer = newTokenizer
	log.Printf("Context Constructor tokenizer updated for model: %s", model)
	return nil
}
