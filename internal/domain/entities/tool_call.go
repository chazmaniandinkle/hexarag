package entities

import (
	"encoding/json"
	"time"
)

// ToolCallStatus represents the status of a tool call
type ToolCallStatus string

const (
	ToolCallStatusPending ToolCallStatus = "pending"
	ToolCallStatusSuccess ToolCallStatus = "success"
	ToolCallStatusError   ToolCallStatus = "error"
)

// ToolCall represents a function call made by the LLM
type ToolCall struct {
	ID        string                 `json:"id"`
	MessageID string                 `json:"message_id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    *ToolCallResult        `json:"result,omitempty"`
	Status    ToolCallStatus         `json:"status"`
	CreatedAt time.Time              `json:"created_at"`
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewToolCall creates a new tool call
func NewToolCall(messageID, name string, arguments map[string]interface{}) *ToolCall {
	return &ToolCall{
		ID:        generateID(),
		MessageID: messageID,
		Name:      name,
		Arguments: arguments,
		Status:    ToolCallStatusPending,
		CreatedAt: time.Now(),
	}
}

// SetResult sets the successful result of the tool call
func (tc *ToolCall) SetResult(data interface{}) {
	tc.Result = &ToolCallResult{
		Data:      data,
		Timestamp: time.Now(),
	}
	tc.Status = ToolCallStatusSuccess
}

// SetError sets an error result for the tool call
func (tc *ToolCall) SetError(err string) {
	tc.Result = &ToolCallResult{
		Error:     err,
		Timestamp: time.Now(),
	}
	tc.Status = ToolCallStatusError
}

// IsCompleted returns true if the tool call has finished (success or error)
func (tc *ToolCall) IsCompleted() bool {
	return tc.Status == ToolCallStatusSuccess || tc.Status == ToolCallStatusError
}

// IsPending returns true if the tool call is still pending
func (tc *ToolCall) IsPending() bool {
	return tc.Status == ToolCallStatusPending
}

// HasError returns true if the tool call resulted in an error
func (tc *ToolCall) HasError() bool {
	return tc.Status == ToolCallStatusError
}

// ArgumentsJSON returns the arguments as a JSON string
func (tc *ToolCall) ArgumentsJSON() (string, error) {
	data, err := json.Marshal(tc.Arguments)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ResultJSON returns the result as a JSON string
func (tc *ToolCall) ResultJSON() (string, error) {
	if tc.Result == nil {
		return "", nil
	}
	data, err := json.Marshal(tc.Result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
