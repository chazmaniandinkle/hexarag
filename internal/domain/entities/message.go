package entities

import (
	"time"
)

// MessageRole represents the role of a message in a conversation
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// Message represents a single message in a conversation
type Message struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversation_id"`
	Role           MessageRole `json:"role"`
	Content        string      `json:"content"`
	ParentID       *string     `json:"parent_id,omitempty"` // For future branching support
	TokenCount     int         `json:"token_count"`
	Model          string      `json:"model,omitempty"`
	ToolCalls      []ToolCall  `json:"tool_calls,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

// NewMessage creates a new message with generated ID and timestamp
func NewMessage(conversationID string, role MessageRole, content string) *Message {
	return &Message{
		ID:             generateID(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		TokenCount:     0, // Will be calculated by tokenizer
		CreatedAt:      time.Now(),
	}
}

// AddToolCall adds a tool call to the message
func (m *Message) AddToolCall(toolCall ToolCall) {
	if m.ToolCalls == nil {
		m.ToolCalls = make([]ToolCall, 0)
	}
	m.ToolCalls = append(m.ToolCalls, toolCall)
}

// SetTokenCount sets the token count for the message
func (m *Message) SetTokenCount(count int) {
	m.TokenCount = count
}

// IsFromUser returns true if the message is from a user
func (m *Message) IsFromUser() bool {
	return m.Role == RoleUser
}

// IsFromAssistant returns true if the message is from an assistant
func (m *Message) IsFromAssistant() bool {
	return m.Role == RoleAssistant
}

// HasToolCalls returns true if the message contains tool calls
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}
