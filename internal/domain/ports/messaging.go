package ports

import (
	"context"
)

// MessageHandler defines a function type for handling incoming messages
type MessageHandler func(ctx context.Context, subject string, data []byte) error

// MessagingPort defines the interface for event bus operations
type MessagingPort interface {
	// Publish sends a message to the specified subject
	Publish(ctx context.Context, subject string, data []byte) error

	// PublishJSON publishes a JSON-serializable object to the subject
	PublishJSON(ctx context.Context, subject string, obj interface{}) error

	// Subscribe listens for messages on the specified subject
	Subscribe(ctx context.Context, subject string, handler MessageHandler) error

	// SubscribeQueue creates a queue subscription for load balancing
	SubscribeQueue(ctx context.Context, subject, queue string, handler MessageHandler) error

	// Unsubscribe stops listening to a subject
	Unsubscribe(ctx context.Context, subject string) error

	// Request sends a request and waits for a response
	Request(ctx context.Context, subject string, data []byte, timeout ...interface{}) ([]byte, error)

	// Close closes the messaging connection
	Close() error

	// Health check
	Ping() error
}

// Standard subjects used across the system
const (
	// Conversation events
	SubjectConversationMessageNew = "conversation.%s.message.new" // conversation_id
	SubjectConversationUpdated    = "conversation.%s.updated"     // conversation_id

	// Inference events
	SubjectInferenceRequest  = "inference.request"
	SubjectInferenceResponse = "inference.response"

	// Tool events
	SubjectToolExecute = "tool.execute"
	SubjectToolResult  = "tool.result"

	// Context construction events
	SubjectContextRequest = "context.request"
	SubjectContextReady   = "context.ready"

	// System events
	SubjectSystemHealth = "system.health"
	SubjectSystemError  = "system.error"
)
