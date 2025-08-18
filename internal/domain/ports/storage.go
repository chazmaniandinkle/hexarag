package ports

import (
	"context"

	"github.com/username/hexarag/internal/domain/entities"
)

// StoragePort defines the interface for persistent storage operations
type StoragePort interface {
	// Message operations
	SaveMessage(ctx context.Context, message *entities.Message) error
	GetMessage(ctx context.Context, id string) (*entities.Message, error)
	GetMessages(ctx context.Context, conversationID string, limit int) ([]*entities.Message, error)
	GetMessagesAfter(ctx context.Context, conversationID string, afterID string, limit int) ([]*entities.Message, error)

	// Conversation operations
	SaveConversation(ctx context.Context, conversation *entities.Conversation) error
	GetConversation(ctx context.Context, id string) (*entities.Conversation, error)
	GetConversations(ctx context.Context, limit int, offset int) ([]*entities.Conversation, error)
	UpdateConversation(ctx context.Context, conversation *entities.Conversation) error
	DeleteConversation(ctx context.Context, id string) error

	// System prompt operations
	SaveSystemPrompt(ctx context.Context, prompt *entities.SystemPrompt) error
	GetSystemPrompt(ctx context.Context, id string) (*entities.SystemPrompt, error)
	GetSystemPrompts(ctx context.Context) ([]*entities.SystemPrompt, error)
	UpdateSystemPrompt(ctx context.Context, prompt *entities.SystemPrompt) error
	DeleteSystemPrompt(ctx context.Context, id string) error

	// Tool call operations
	SaveToolCall(ctx context.Context, toolCall *entities.ToolCall) error
	GetToolCall(ctx context.Context, id string) (*entities.ToolCall, error)
	GetToolCallsForMessage(ctx context.Context, messageID string) ([]*entities.ToolCall, error)
	UpdateToolCall(ctx context.Context, toolCall *entities.ToolCall) error

	// Event operations (for event sourcing)
	SaveEvent(ctx context.Context, conversationID, eventType string, payload map[string]interface{}) error
	GetEvents(ctx context.Context, conversationID string, limit int) ([]Event, error)

	// Health check
	Ping(ctx context.Context) error

	// Migration support
	Migrate(ctx context.Context) error
}

// Event represents a stored event for event sourcing
type Event struct {
	ID             string                 `json:"id"`
	ConversationID string                 `json:"conversation_id"`
	EventType      string                 `json:"event_type"`
	Payload        map[string]interface{} `json:"payload"`
	CreatedAt      string                 `json:"created_at"` // ISO 8601 timestamp
}
