package entities

import (
	"time"
)

// SystemPrompt represents a reusable system prompt
type SystemPrompt struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewSystemPrompt creates a new system prompt
func NewSystemPrompt(name, content string) *SystemPrompt {
	now := time.Now()
	return &SystemPrompt{
		ID:        generateID(),
		Name:      name,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Update modifies the system prompt content and name
func (sp *SystemPrompt) Update(name, content string) {
	sp.Name = name
	sp.Content = content
	sp.UpdatedAt = time.Now()
}

// IsEmpty returns true if the prompt content is empty
func (sp *SystemPrompt) IsEmpty() bool {
	return len(sp.Content) == 0
}

// DefaultSystemPrompts returns a set of default system prompts
func DefaultSystemPrompts() []*SystemPrompt {
	return []*SystemPrompt{
		{
			ID:        "default",
			Name:      "Default Assistant",
			Content:   "You are a helpful AI assistant. Respond clearly and concisely to user questions.",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "creative",
			Name:      "Creative Writer",
			Content:   "You are a creative writing assistant. Help users with storytelling, poetry, and creative expression.",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "technical",
			Name:      "Technical Assistant",
			Content:   "You are a technical assistant specializing in programming, software development, and technical problem-solving. Provide accurate, detailed explanations with code examples when appropriate.",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}
