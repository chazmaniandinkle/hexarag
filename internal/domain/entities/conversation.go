package entities

import (
	"time"
)

// Conversation represents a chat conversation
type Conversation struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	SystemPromptID string    `json:"system_prompt_id"`
	Model          string    `json:"model,omitempty"` // Preferred model for this conversation
	MessageIDs     []string  `json:"message_ids"`     // Ordered list of message IDs
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewConversation creates a new conversation with the given system prompt
func NewConversation(title, systemPromptID string) *Conversation {
	now := time.Now()
	return &Conversation{
		ID:             generateID(),
		Title:          title,
		SystemPromptID: systemPromptID,
		MessageIDs:     make([]string, 0),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// AddMessage adds a message ID to the conversation
func (c *Conversation) AddMessage(messageID string) {
	c.MessageIDs = append(c.MessageIDs, messageID)
	c.UpdatedAt = time.Now()
}

// SetSystemPrompt changes the system prompt for the conversation
func (c *Conversation) SetSystemPrompt(systemPromptID string) {
	c.SystemPromptID = systemPromptID
	c.UpdatedAt = time.Now()
}

// SetTitle updates the conversation title
func (c *Conversation) SetTitle(title string) {
	c.Title = title
	c.UpdatedAt = time.Now()
}

// SetModel updates the preferred model for this conversation
func (c *Conversation) SetModel(model string) {
	c.Model = model
	c.UpdatedAt = time.Now()
}

// MessageCount returns the number of messages in the conversation
func (c *Conversation) MessageCount() int {
	return len(c.MessageIDs)
}

// IsEmpty returns true if the conversation has no messages
func (c *Conversation) IsEmpty() bool {
	return len(c.MessageIDs) == 0
}
