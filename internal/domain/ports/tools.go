package ports

import (
	"context"
)

// ToolPort defines the interface for tool execution
type ToolPort interface {
	// Execute runs a tool with the given arguments
	Execute(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)

	// GetAvailableTools returns the list of available tools
	GetAvailableTools(ctx context.Context) ([]Tool, error)

	// GetTool returns information about a specific tool
	GetTool(ctx context.Context, name string) (*Tool, error)

	// Health check
	Ping(ctx context.Context) error
}

// Tool represents a function/tool that can be called by the LLM
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function definition
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success  bool                   `json:"success"`
	Data     interface{}            `json:"data,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolExecutionRequest represents a request to execute a tool
type ToolExecutionRequest struct {
	ToolCallID     string                 `json:"tool_call_id"`
	Name           string                 `json:"name"`
	Arguments      map[string]interface{} `json:"arguments"`
	MessageID      string                 `json:"message_id"`
	ConversationID string                 `json:"conversation_id"`
}

// ToolExecutionResponse represents the response from tool execution
type ToolExecutionResponse struct {
	ToolCallID     string      `json:"tool_call_id"`
	Result         *ToolResult `json:"result"`
	MessageID      string      `json:"message_id"`
	ConversationID string      `json:"conversation_id"`
}
